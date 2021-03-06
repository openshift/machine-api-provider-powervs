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

// EventUser event user
//
// swagger:model EventUser
type EventUser struct {

	// Email of the User
	Email string `json:"email,omitempty"`

	// Name of the User
	Name string `json:"name,omitempty"`

	// ID of user who created/caused the event
	// Required: true
	UserID *string `json:"userID"`
}

// Validate validates this event user
func (m *EventUser) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateUserID(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *EventUser) validateUserID(formats strfmt.Registry) error {

	if err := validate.Required("userID", "body", m.UserID); err != nil {
		return err
	}

	return nil
}

// ContextValidate validates this event user based on context it is used
func (m *EventUser) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *EventUser) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *EventUser) UnmarshalBinary(b []byte) error {
	var res EventUser
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
