package simpleblock

const (
	TaskQueue        = "simple-block-v1"
	WorkflowName     = "SimpleBlockWorkflowV1"
	CatalogQueryName = "item_catalog"
)

type Item struct {
	ItemID      string  `json:"itemId"`
	Description string  `json:"description"`
	Image       string  `json:"image"`
	Price       float64 `json:"price"`
}

type AddItemCommand = Item

type ItemAdded = Item

type ItemAddRejected struct {
	ItemID string `json:"itemId"`
	Reason string `json:"reason"`
}

type AddItemResult struct {
	Event    string           `json:"event"`
	Added    *ItemAdded       `json:"itemAdded,omitempty"`
	Rejected *ItemAddRejected `json:"itemAddRejected,omitempty"`
}

type ItemCatalog struct {
	Items []Item `json:"items"`
}

type WorkflowInput struct {
	Command     AddItemCommand `json:"command"`
	PriorEvents []string       `json:"priorEvents,omitempty"`
}

type WorkflowResult struct {
	Decision AddItemResult `json:"decision"`
	Catalog  ItemCatalog   `json:"catalog"`
}
