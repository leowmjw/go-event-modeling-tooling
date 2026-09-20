package simpleblock

import "go.temporal.io/sdk/workflow"

func Workflow(ctx workflow.Context, input WorkflowInput) (WorkflowResult, error) {
	result, err := DecideAddItem(input.Command)
	if err != nil {
		return WorkflowResult{}, err
	}
	catalog := ProjectItemCatalog(result)
	if err := workflow.SetQueryHandler(ctx, CatalogQueryName, func() (ItemCatalog, error) { return catalog, nil }); err != nil {
		return WorkflowResult{}, err
	}
	return WorkflowResult{Decision: result, Catalog: catalog}, nil
}
