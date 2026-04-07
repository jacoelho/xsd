package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateTopLevelElementReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if decl.IsReference {
			refDecl, exists := sch.ElementDecls[decl.Name]
			if !exists {
				errs = append(errs, fmt.Errorf("element reference %s does not exist", decl.Name))
			} else if refDecl.IsReference {
				errs = append(errs, fmt.Errorf("element reference %s points to another reference %s (circular or invalid)", qname, decl.Name))
			}
		}
	}

	return errs
}

func validateContentElementReferences(sch *parser.Schema, elementRefsInContent []*model.ElementDecl) []error {
	var errs []error

	for _, elemRef := range elementRefsInContent {
		refDecl, exists := sch.ElementDecls[elemRef.Name]
		if !exists {
			errs = append(errs, fmt.Errorf("element reference %s in content model does not exist", elemRef.Name))
		} else if refDecl.IsReference {
			errs = append(errs, fmt.Errorf("element reference %s in content model points to another reference (circular or invalid)", elemRef.Name))
		}
	}

	return errs
}

func validateElementDeclarationReferences(sch *parser.Schema, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if decl.Type != nil {
			origin := sch.ElementOrigins[qname]
			if err := validateTypeReferenceFromTypeAtLocation(sch, decl.Type, qname.Namespace, origin); err != nil {
				errs = append(errs, fmt.Errorf("element %s: %w", qname, err))
			}
		}

		if err := validateElementValueConstraints(sch, decl); err != nil {
			errs = append(errs, fmt.Errorf("element %s: %w", qname, err))
		}

		errs = append(errs, validateElementSubstitutionGroupReferences(sch, qname, decl)...)
		errs = append(errs, validateElementIdentityConstraintReferences(sch, qname, decl, allConstraints)...)
	}

	return errs
}

func validateElementSubstitutionGroupReferences(sch *parser.Schema, qname model.QName, decl *model.ElementDecl) []error {
	if decl.SubstitutionGroup == (model.QName{}) {
		return nil
	}

	headDecl, exists := sch.ElementDecls[decl.SubstitutionGroup]
	if !exists {
		return []error{
			fmt.Errorf("element %s substitutionGroup %s does not exist", qname, decl.SubstitutionGroup),
		}
	}

	var errs []error
	if err := validateSubstitutionGroupDerivation(sch, qname, decl, headDecl); err != nil {
		errs = append(errs, err)
	}
	if err := validateSubstitutionGroupFinal(sch, qname, decl, headDecl); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func validateElementIdentityConstraintReferences(sch *parser.Schema, qname model.QName, decl *model.ElementDecl, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	if err := ValidateKeyrefConstraints(qname, decl.Constraints, allConstraints); err != nil {
		errs = append(errs, err...)
	}

	for _, constraint := range decl.Constraints {
		if err := ValidateIdentityConstraintResolution(sch, constraint, decl); err != nil {
			errs = append(errs, fmt.Errorf("element %s identity constraint '%s': %w", qname, constraint.Name, err))
		}
	}

	return errs
}
