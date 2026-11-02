package scope

import (
	"context"
	"testing"
)

func TestSchemaContext(t *testing.T) {
	ctx := context.Background()
	if got := SchemaFrom(ctx); got != "" {
		t.Fatalf("SchemaFrom(empty) = %q, want empty", got)
	}
	if got := SchemaFrom(WithSchema(ctx, "tenant_a")); got != "tenant_a" {
		t.Fatalf("SchemaFrom(WithSchema) = %q, want tenant_a", got)
	}
}
