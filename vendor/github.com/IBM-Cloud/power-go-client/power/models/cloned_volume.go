// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// ClonedVolume cloned volume
//
// swagger:model ClonedVolume
type ClonedVolume struct {

	// ID of the new cloned volume
	ClonedVolumeID string `json:"clonedVolumeID,omitempty"`

	// ID of the source volume to be cloned
	SourceVolumeID string `json:"sourceVolumeID,omitempty"`
}

// Validate validates this cloned volume
func (m *ClonedVolume) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this cloned volume based on context it is used
func (m *ClonedVolume) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *ClonedVolume) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ClonedVolume) UnmarshalBinary(b []byte) error {
	var res ClonedVolume
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
