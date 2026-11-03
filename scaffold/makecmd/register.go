package makecmd

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"path"
	"strconv"
)

// Registration edits are located in the parsed Go AST. Insertions use token
// positions, so existing comments (including inline comments) remain in place.
// Unsupported shapes are rejected instead of rewriting user control flow.
type editor struct {
	source []byte
	file   *ast.File
	fset   *token.FileSet
	name   string
}

func (op *operation) edit(relative string, run func(*editor) error) error {
	ch, err := op.load(relative)
	if err != nil {
		return err
	}
	e := &editor{source: append([]byte{}, ch.after...), name: relative}
	if err = e.parse(); err != nil {
		return err
	}
	if err = run(e); err != nil {
		return fmt.Errorf("register in %s: %w", relative, err)
	}
	formatted, err := format.Source(e.source)
	if err != nil {
		return fmt.Errorf("format registration %s: %w", relative, err)
	}
	ch.after = formatted
	return nil
}
func (e *editor) parse() error {
	e.fset = token.NewFileSet()
	f, err := parser.ParseFile(e.fset, e.name, e.source, parser.ParseComments)
	e.file = f
	return err
}
func (e *editor) offset(pos token.Pos) int { return e.fset.Position(pos).Offset }
func (e *editor) insert(pos token.Pos, text string) error {
	offset := e.offset(pos)
	out := make([]byte, 0, len(e.source)+len(text))
	out = append(out, e.source[:offset]...)
	out = append(out, text...)
	out = append(out, e.source[offset:]...)
	e.source = out
	return e.parse()
}
func printed(node any) string {
	var b bytes.Buffer
	_ = format.Node(&b, token.NewFileSet(), node)
	return b.String()
}
func expression(text string) (ast.Expr, error) { return parser.ParseExpr(text) }
func (e *editor) importName(importPath, preferred string) (string, error) {
	count := 0
	for _, spec := range e.file.Imports {
		value, _ := strconv.Unquote(spec.Path.Value)
		if value == importPath {
			count++
		}
	}
	if count > 1 {
		return "", fmt.Errorf("ambiguous duplicate import %s", importPath)
	}
	for _, spec := range e.file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", err
		}
		if value != importPath {
			continue
		}
		if spec.Name != nil {
			if spec.Name.Name == "." || spec.Name.Name == "_" {
				return "", fmt.Errorf("import %s must have a usable name", importPath)
			}
			return spec.Name.Name, nil
		}
		return path.Base(importPath), nil
	}
	used := map[string]bool{}
	ast.Inspect(e.file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	alias := preferred
	for i := 2; used[alias]; i++ {
		alias = fmt.Sprintf("%s%d", preferred, i)
	}
	decl := &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{&ast.ImportSpec{Name: ast.NewIdent(alias), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(importPath)}}}}
	// Insert after the package clause, before any user declarations/comments.
	pos := e.file.Name.End()
	if end := bytes.IndexByte(e.source[e.offset(pos):], '\n'); end >= 0 {
		pos += token.Pos(end)
	}
	err := e.insert(pos, "\n\n"+printed(decl)+"\n")
	return alias, err
}
func (e *editor) imported(local, importPath string) bool {
	for _, spec := range e.file.Imports {
		p, _ := strconv.Unquote(spec.Path.Value)
		if p != importPath {
			continue
		}
		if spec.Name == nil {
			return local == path.Base(p)
		}
		return spec.Name.Name == local
	}
	return false
}
func (e *editor) isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && e.imported(id.Name, pkg)
}
func (e *editor) appendList(close token.Pos, items []ast.Expr, newExpr ast.Expr) error {
	for _, item := range items {
		if printed(item) == printed(newExpr) {
			return nil
		}
	}
	offset := e.offset(close)
	payload := "\n" + printed(newExpr) + ",\n"
	lineStart := bytes.LastIndexByte(e.source[:offset], '\n') + 1
	if len(bytes.TrimSpace(e.source[lineStart:offset])) == 0 {
		offset = lineStart
		close = token.Pos(offset + 1)
		payload = printed(newExpr) + ",\n"
	}
	if len(items) > 0 {
		last := items[len(items)-1]
		start := e.offset(last.End())
		var scan scanner.Scanner
		scan.Init(token.NewFileSet().AddFile("tail", -1, offset-start), e.source[start:offset], nil, 0)
		_, tok, _ := scan.Scan()
		if tok != token.COMMA {
			// Apply both AST-located insertions before parsing; either insertion alone
			// can temporarily violate Go's trailing-comma rule.
			out := make([]byte, 0, len(e.source)+len(payload)+1)
			out = append(out, e.source[:start]...)
			out = append(out, ',')
			out = append(out, e.source[start:offset]...)
			out = append(out, payload...)
			out = append(out, e.source[offset:]...)
			e.source = out
			return e.parse()
		}
	}
	return e.insert(close, payload)
}
func (e *editor) set(name string) (*ast.CallExpr, error) {
	var found *ast.CallExpr
	for _, decl := range e.file.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range g.Specs {
			v, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, id := range v.Names {
				if id.Name != name {
					continue
				}
				if found != nil || g.Tok != token.VAR || len(v.Names) != 1 || len(v.Values) != 1 {
					return nil, fmt.Errorf("%s must be one var initialized by wire.NewSet", name)
				}
				call, ok := v.Values[0].(*ast.CallExpr)
				if !ok || !e.isSelector(call.Fun, "github.com/goforj/wire", "NewSet") {
					return nil, fmt.Errorf("%s must use wire.NewSet", name)
				}
				found = call
			}
		}
	}
	if found == nil {
		return nil, fmt.Errorf("missing %s = wire.NewSet(...)", name)
	}
	return found, nil
}
func (e *editor) addSet(name, text string) error {
	set, err := e.set(name)
	if err != nil {
		return err
	}
	expr, err := expression(text)
	if err != nil {
		return err
	}
	return e.appendList(set.Rparen, set.Args, expr)
}
func (e *editor) function(name string) (*ast.FuncDecl, error) {
	var found *ast.FuncDecl
	for _, d := range e.file.Decls {
		f, ok := d.(*ast.FuncDecl)
		if ok && f.Recv == nil && f.Name.Name == name {
			if found != nil {
				return nil, fmt.Errorf("ambiguous function %s", name)
			}
			found = f
		}
	}
	if found == nil || found.Body == nil {
		return nil, fmt.Errorf("missing function %s", name)
	}
	return found, nil
}
func (e *editor) literalFunction(name string) (*ast.FuncDecl, *ast.CompositeLit, error) {
	f, err := e.function(name)
	if err != nil {
		return nil, nil, err
	}
	if len(f.Body.List) != 1 {
		return nil, nil, fmt.Errorf("%s must contain one direct return of a literal", name)
	}
	ret, ok := f.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil, nil, fmt.Errorf("%s must directly return a literal", name)
	}
	lit, ok := ret.Results[0].(*ast.CompositeLit)
	if !ok {
		return nil, nil, fmt.Errorf("%s must directly return a literal", name)
	}
	return f, lit, nil
}
func (e *editor) addLiteral(name, text, kind string) error {
	f, lit, err := e.literalFunction(name)
	if err != nil {
		return err
	}
	switch kind {
	case "models", "seeders":
		typeName := "Models"
		if kind == "seeders" {
			typeName = "Seeders"
		}
		if !e.isSelector(lit.Type, "gitlab.com/skyrix-lib/framework/database/dbcmd", typeName) {
			return fmt.Errorf("%s must return dbcmd.%s{}", name, typeName)
		}
		if f.Type.Results == nil || len(f.Type.Results.List) != 1 || !e.isSelector(f.Type.Results.List[0].Type, "gitlab.com/skyrix-lib/framework/database/dbcmd", typeName) {
			return fmt.Errorf("unsupported %s return type", name)
		}
	case "commands":
		array, ok := lit.Type.(*ast.ArrayType)
		if !ok || array.Len != nil {
			return fmt.Errorf("%s must return []*console.Command{}", name)
		}
		star, ok := array.Elt.(*ast.StarExpr)
		if !ok || !e.isSelector(star.X, "gitlab.com/skyrix-lib/framework/cobra/console", "Command") {
			return fmt.Errorf("%s must return []*console.Command{}", name)
		}
	}
	for _, item := range lit.Elts {
		if _, keyed := item.(*ast.KeyValueExpr); keyed {
			return fmt.Errorf("%s keyed registry entries are unsupported", name)
		}
	}
	expr, err := expression(text)
	if err != nil {
		return err
	}
	return e.appendList(lit.Rbrace, lit.Elts, expr)
}
func (e *editor) commandParam(name string, db bool) (string, error) {
	f, _, err := e.literalFunction(name)
	if err != nil {
		return "", err
	}
	if f.Type.Params == nil || len(f.Type.Params.List) != 1 || len(f.Type.Params.List[0].Names) != 1 {
		return "", fmt.Errorf("%s requires one named graph parameter", name)
	}
	field := f.Type.Params.List[0]
	star, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return "", fmt.Errorf("%s requires *console.Graph", name)
	}
	index, ok := star.X.(*ast.IndexExpr)
	if !ok || !e.isSelector(index.X, "gitlab.com/skyrix-lib/framework/cobra/console", "Graph") {
		return "", fmt.Errorf("%s requires *console.Graph", name)
	}
	target, ok := index.Index.(*ast.StarExpr)
	if !ok {
		return "", fmt.Errorf("unsupported %s graph target", name)
	}
	if db {
		if !e.isSelector(target.X, "gitlab.com/skyrix-lib/framework/database/dbcmd", "Deps") {
			return "", fmt.Errorf("%s requires database graph", name)
		}
	} else if id, ok := target.X.(*ast.Ident); !ok || id.Name != "Deps" {
		return "", fmt.Errorf("%s requires application Deps", name)
	}
	return field.Names[0].Name, nil
}
func (e *editor) addField(structName, fieldName, typeText string) error {
	var target *ast.StructType
	for _, decl := range e.file.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range g.Specs {
			t, ok := spec.(*ast.TypeSpec)
			if !ok || t.Name.Name != structName {
				continue
			}
			s, ok := t.Type.(*ast.StructType)
			if !ok || target != nil {
				return fmt.Errorf("unsupported struct %s", structName)
			}
			target = s
		}
	}
	if target == nil {
		return fmt.Errorf("missing struct %s", structName)
	}
	typ, err := expression(typeText)
	if err != nil {
		return err
	}
	for _, field := range target.Fields.List {
		for _, id := range field.Names {
			if id.Name == fieldName {
				if printed(field.Type) == printed(typ) {
					return nil
				}
				return fmt.Errorf("field %s.%s already has another type", structName, fieldName)
			}
		}
	}
	field := &ast.Field{Names: []*ast.Ident{ast.NewIdent(fieldName)}, Type: typ}
	return e.insert(target.Fields.Closing, "\n"+printed(field.Names[0])+" "+printed(field.Type)+"\n")
}
func (e *editor) addSeeder(name, typeText string) error {
	f, _, err := e.literalFunction("Seeders")
	if err != nil {
		return err
	}
	typ, err := expression(typeText)
	if err != nil {
		return err
	}
	for _, field := range f.Type.Params.List {
		for _, id := range field.Names {
			if id.Name == name {
				return fmt.Errorf("seeder parameter %s already exists", name)
			}
		}
	}
	// Prevent an unnamed parameter list becoming a mixed declaration.
	for _, field := range f.Type.Params.List {
		if len(field.Names) == 0 {
			return fmt.Errorf("Seeders parameters must be named")
		}
	}
	field := &ast.Field{Names: []*ast.Ident{ast.NewIdent(name)}, Type: typ}
	payload := printed(field.Names[0]) + " " + printed(field.Type)
	if len(f.Type.Params.List) > 0 {
		last := f.Type.Params.List[len(f.Type.Params.List)-1]
		tail := e.source[e.offset(last.End()):e.offset(f.Type.Params.Closing)]
		var scan scanner.Scanner
		scan.Init(token.NewFileSet().AddFile("params", -1, len(tail)), tail, nil, 0)
		_, tok, _ := scan.Scan()
		if tok == token.COMMA {
			payload = "\n" + payload + ",\n"
		} else {
			payload = ", " + payload
		}
	}
	if err := e.insert(f.Type.Params.Closing, payload); err != nil {
		return err
	}
	return e.addLiteral("Seeders", name, "seeders")
}
func (e *editor) addJobRegistration(field string) error {
	f, err := e.function("ProvideRegisteredJobsRegistry")
	if err != nil {
		return err
	}
	if len(f.Type.Params.List) != 2 || len(f.Type.Params.List[0].Names) != 1 || len(f.Type.Params.List[1].Names) != 1 {
		return fmt.Errorf("unsupported jobs registry signature")
	}
	regType, ok := f.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok || !e.isSelector(regType.X, "gitlab.com/skyrix-lib/framework/engine/jobs", "RuntimeRegistry") {
		return fmt.Errorf("jobs registry requires *jobs.RuntimeRegistry")
	}
	allType, ok := f.Type.Params.List[1].Type.(*ast.StarExpr)
	if !ok || printed(allType.X) != "Jobs" {
		return fmt.Errorf("jobs registry requires *Jobs bundle")
	}
	reg, all := f.Type.Params.List[0].Names[0].Name, f.Type.Params.List[1].Names[0].Name
	if len(f.Body.List) == 0 {
		return fmt.Errorf("empty jobs registry")
	}
	last, ok := f.Body.List[len(f.Body.List)-1].(*ast.ReturnStmt)
	if !ok || len(last.Results) != 1 || printed(last.Results[0]) != reg {
		return fmt.Errorf("jobs registry must end with return %s", reg)
	}
	target := reg + ".Register(" + all + "." + field + ")"
	for _, stmt := range f.Body.List[:len(f.Body.List)-1] {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			return fmt.Errorf("unsupported jobs registry statement")
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok || printed(call.Fun) != reg+".Register" || len(call.Args) != 1 {
			return fmt.Errorf("unsupported jobs registry call")
		}
		if printed(call) == target {
			return nil
		}
	}
	call, err := expression(target)
	if err != nil {
		return err
	}
	return e.insert(last.Pos(), printed(&ast.ExprStmt{X: call})+"\n")
}

func (e *editor) requireBundle(setName, typeName string) error {
	set, err := e.set(setName)
	if err != nil {
		return err
	}
	matches := 0
	for _, arg := range set.Args {
		call, ok := arg.(*ast.CallExpr)
		if !ok || !e.isSelector(call.Fun, "github.com/goforj/wire", "Struct") {
			continue
		}
		if len(call.Args) != 2 || printed(call.Args[0]) != "new("+typeName+")" {
			continue
		}
		value, ok := call.Args[1].(*ast.BasicLit)
		if !ok {
			continue
		}
		text, _ := strconv.Unquote(value.Value)
		if text == "*" {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("%s must contain one wire.Struct(new(%s), \"*\") bundle", setName, typeName)
	}
	return nil
}
