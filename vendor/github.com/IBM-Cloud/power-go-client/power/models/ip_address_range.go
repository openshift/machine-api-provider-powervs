// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// IPAddressRange IP address range
//
// swagger:model IPAddressRange
type IPAddressRange struct {

	// Ending IP Address
	// Required: true
	EndingIPAddress *string `json:"endingIPAddress"`

	// Starting IP Address
	// Required: true
	StartingIPAddress *string `json:"startingIPAddress"`
}

// Validate validates this IP address range
func (m *IPAddressRange) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateEndingIPAddress(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateStartingIPAddress(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *IPAddressRange) validateEndingIPAddress(formats strfmt.Registry) error {

	if err := validate.Required("endingIPAddress", "body", m.EndingIPAddress); err != nil {
		return err
	}

	return nil
}

func (m *IPAddressRange) validateStartingIPAddress(formats strfmt.Registry) error {

	if err := validate.Required("startingIPAddress", "body", m.StartingIPAddress); err != nil {
		return err
	}

	return nil
}

// ContextValidate validates this IP address range based on context it is used
func (m *IPAddressRange) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *IPAddressRange) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *IPAddressRange) UnmarshalBinary(b []byte) error {
	var res IPAddressRange
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
