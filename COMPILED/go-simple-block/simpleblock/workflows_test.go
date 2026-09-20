package simpleblock

import (
	"testing"

	"go.temporal.io/sdk/testsuite"
)

func TestWorkflowGWTScenarios(t *testing.T) {
	tests := []struct {
		name    string
		command AddItemCommand
		event   string
	}{
		{name: "add a valid item", command: AddItemCommand{ItemID: "item-1", Description: "john", Image: "avatar_john", Price: 20.4}, event: "ItemAdded"},
		{name: "reject an item with a non-positive price", command: AddItemCommand{ItemID: "item-2", Description: "jane", Image: "avatar_jane", Price: 0}, event: "ItemAddRejected"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := new(testsuite.WorkflowTestSuite).NewTestWorkflowEnvironment()
			env.ExecuteWorkflow(Workflow, WorkflowInput{Command: tc.command})
			if err := env.GetWorkflowError(); err != nil {
				t.Fatal(err)
			}
			var result WorkflowResult
			if err := env.GetWorkflowResult(&result); err != nil {
				t.Fatal(err)
			}
			if result.Decision.Event != tc.event {
				t.Fatalf("event = %q, want %q", result.Decision.Event, tc.event)
			}
			if tc.event == "ItemAdded" {
				if len(result.Catalog.Items) != 1 || result.Catalog.Items[0] != Item(tc.command) {
					t.Fatalf("catalog = %#v", result.Catalog)
				}
			} else if len(result.Catalog.Items) != 0 || result.Decision.Rejected == nil || result.Decision.Rejected.Reason != "price_must_be_positive" {
				t.Fatalf("rejection = %#v", result)
			}
		})
	}
}
