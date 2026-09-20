package simpleblock

import "fmt"

func DecideAddItem(command AddItemCommand) (AddItemResult, error) {
	if command.ItemID == "" || command.Description == "" || command.Image == "" {
		return AddItemResult{}, fmt.Errorf("itemId, description, and image are required")
	}
	if command.Price <= 0 {
		return AddItemResult{Event: "ItemAddRejected", Rejected: &ItemAddRejected{ItemID: command.ItemID, Reason: "price_must_be_positive"}}, nil
	}
	added := ItemAdded(command)
	return AddItemResult{Event: "ItemAdded", Added: &added}, nil
}

func ProjectItemCatalog(result AddItemResult) ItemCatalog {
	if result.Added == nil {
		return ItemCatalog{Items: []Item{}}
	}
	return ItemCatalog{Items: []Item{Item(*result.Added)}}
}
