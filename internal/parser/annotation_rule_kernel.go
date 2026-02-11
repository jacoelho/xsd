package parser

import "errors"

func handleSingleLeadingAnnotation(name string, seenAnnotation *bool, seenNonAnnotation bool, duplicateMessage string, orderingMessage string) (bool, error) {
	if name != "annotation" {
		return false, nil
	}
	if *seenAnnotation {
		return true, errors.New(duplicateMessage)
	}
	if seenNonAnnotation {
		return true, errors.New(orderingMessage)
	}
	*seenAnnotation = true
	return true, nil
}
