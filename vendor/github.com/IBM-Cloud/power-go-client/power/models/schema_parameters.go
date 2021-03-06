// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// SchemaParameters schema parameters
//
// swagger:model SchemaParameters
type SchemaParameters struct {

	// parameters
	Parameters JSONSchemaObject `json:"parameters,omitempty"`
}

// Validate validates this schema parameters
func (m *SchemaParameters) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this schema parameters based on context it is used
func (m *SchemaParameters) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *SchemaParameters) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *SchemaParameters) UnmarshalBinary(b []byte) error {
	var res SchemaParameters
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
