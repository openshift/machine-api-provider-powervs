// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// PVMInstanceHealth PVM's health status details
//
// swagger:model PVMInstanceHealth
type PVMInstanceHealth struct {

	// Date/Time of PVM last health status change
	LastUpdate string `json:"lastUpdate,omitempty"`

	// The health status reason, if any
	Reason string `json:"reason,omitempty"`

	// The PVM's health status value
	Status string `json:"status,omitempty"`
}

// Validate validates this p VM instance health
func (m *PVMInstanceHealth) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this p VM instance health based on context it is used
func (m *PVMInstanceHealth) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *PVMInstanceHealth) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *PVMInstanceHealth) UnmarshalBinary(b []byte) error {
	var res PVMInstanceHealth
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
