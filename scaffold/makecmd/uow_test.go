package makecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUOWProjectStubAndConflicts(t *testing.T) {
	root := registrationProject(t)
	maker := testMaker(root)
	if _, err := maker.generate("domain", "sales", false); err != nil {
		t.Fatal(err)
	}
	if _, err := maker.generate("stubs", "", false); err != nil {
		t.Fatal(err)
	}
	stub := readTestFile(t, filepath.Join(root, "stubs/skyrix/uow.stub"))
	writeProject(t, root, "stubs/skyrix/uow.stub", strings.Replace(stub, "// Execute is", "// Project {{.Module}}\n// Execute is", 1))
	if _, err := maker.generate("uow", "sales/CreateOrder", false); err != nil {
		t.Fatal(err)
	}
	source := readTestFile(t, filepath.Join(root, "internal/domain/sales/unitOfWork/createOrderUOW.go"))
	if !strings.Contains(source, "// Project "+fixtureModule) || !strings.Contains(source, "WithinTransaction") {
		t.Fatal(source)
	}
	provider := readTestFile(t, filepath.Join(root, "internal/domain/sales/provider.go"))
	if strings.Count(provider, "unitofwork.NewCreateOrderUOW") != 1 {
		t.Fatal(provider)
	}
	golden(t, "uow-provider", provider)
	before := projectSnapshot(t, root)
	for _, raw := range []string{"sales/CreateOrder", "sales/../Escape", "sales//bad", "sales/type", "CreateOrder", "missing/CreateOrder"} {
		if _, err := maker.generate("uow", raw, false); err == nil {
			t.Fatalf("accepted %q", raw)
		}
		assertUnchanged(t, root, before)
	}
	// Different filename declaring the same generated symbol is also protected.
	writeProject(t, root, "internal/domain/sales/unitOfWork/user.go", "package unitofwork\ntype ReservedUOW struct{}\n")
	before = projectSnapshot(t, root)
	if _, err := maker.generate("uow", "sales/Reserved", false); err == nil {
		t.Fatal("symbol conflict accepted")
	}
	assertUnchanged(t, root, before)
	for _, bad := range []string{"{{.Unknown}}", "package unitofwork {"} {
		writeProject(t, root, "stubs/skyrix/uow.stub", bad)
		before = projectSnapshot(t, root)
		if _, err := maker.generate("uow", "sales/Bad", false); err == nil {
			t.Fatal("invalid stub accepted")
		}
		assertUnchanged(t, root, before)
	}
}

func TestUOWRejectsSymlinkAndPreservesAST(t *testing.T) {
	root := registrationProject(t)
	maker := testMaker(root)
	if _, err := maker.generate("domain", "orders", false); err != nil {
		t.Fatal(err)
	}
	writeProject(t, root, "internal/domain/orders/provider.go", `package orders
import di "github.com/goforj/wire"
import existing "`+fixtureModule+`/internal/domain/orders/unitOfWork"
var UserSet = di.NewSet()
var ProviderSet = di.NewSet(UserSet /* keep */, existing.NewUserUOW)
`)
	writeProject(t, root, "internal/domain/orders/unitOfWork/user.go", "package unitofwork\ntype UserUOW struct{}\nfunc NewUserUOW()*UserUOW{return &UserUOW{}}\n")
	if _, err := maker.generate("uow", "orders/Create", false); err != nil {
		t.Fatal(err)
	}
	provider := readTestFile(t, filepath.Join(root, "internal/domain/orders/provider.go"))
	if !strings.Contains(provider, "existing.NewCreateUOW") || !strings.Contains(provider, "/* keep */") || strings.Count(provider, "/unitOfWork\"") != 1 {
		t.Fatal(provider)
	}
	// Repeating AST registration itself is idempotent (file generation conflicts).
	op, err := newOperation(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := op.edit("internal/domain/orders/provider.go", func(e *editor) error { return e.addSet("ProviderSet", "existing.NewCreateUOW") }); err != nil {
		t.Fatal(err)
	}
	if string(op.files["internal/domain/orders/provider.go"].after) != provider {
		t.Fatal("duplicate AST registration")
	}
	before := projectSnapshot(t, root)
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "stubs-link")); err != nil {
		t.Fatal(err)
	}
	maker.DomainDir = "stubs-link"
	if _, err := maker.generate("uow", "orders/Escape", false); err == nil {
		t.Fatal("symlink accepted")
	}
	if got, err := os.Readlink(filepath.Join(root, "stubs-link")); err != nil || got != target {
		t.Fatal("symlink changed")
	}
	if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
	if err := os.Remove(filepath.Join(root, "stubs-link")); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, root, before)
}
