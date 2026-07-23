package scope

type Scope int

const (
	Tenant Scope = iota
	Main
)

type SchemaScoped interface {
	SchemaScope() Scope
}

type MainModel struct{}

func (MainModel) SchemaScope() Scope { return Main }

type TenantModel struct{}

func (TenantModel) SchemaScope() Scope { return Tenant }

type TableBasable interface {
	TableBase() string
}
