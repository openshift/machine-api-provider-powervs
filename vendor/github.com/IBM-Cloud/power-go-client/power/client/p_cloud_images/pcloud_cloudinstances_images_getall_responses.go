// Code generated by go-swagger; DO NOT EDIT.

package p_cloud_images

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/IBM-Cloud/power-go-client/power/models"
)

// PcloudCloudinstancesImagesGetallReader is a Reader for the PcloudCloudinstancesImagesGetall structure.
type PcloudCloudinstancesImagesGetallReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *PcloudCloudinstancesImagesGetallReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewPcloudCloudinstancesImagesGetallOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := NewPcloudCloudinstancesImagesGetallBadRequest()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 401:
		result := NewPcloudCloudinstancesImagesGetallUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewPcloudCloudinstancesImagesGetallNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 500:
		result := NewPcloudCloudinstancesImagesGetallInternalServerError()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewPcloudCloudinstancesImagesGetallOK creates a PcloudCloudinstancesImagesGetallOK with default headers values
func NewPcloudCloudinstancesImagesGetallOK() *PcloudCloudinstancesImagesGetallOK {
	return &PcloudCloudinstancesImagesGetallOK{}
}

/* PcloudCloudinstancesImagesGetallOK describes a response with status code 200, with default header values.

OK
*/
type PcloudCloudinstancesImagesGetallOK struct {
	Payload *models.Images
}

func (o *PcloudCloudinstancesImagesGetallOK) Error() string {
	return fmt.Sprintf("[GET /pcloud/v1/cloud-instances/{cloud_instance_id}/images][%d] pcloudCloudinstancesImagesGetallOK  %+v", 200, o.Payload)
}
func (o *PcloudCloudinstancesImagesGetallOK) GetPayload() *models.Images {
	return o.Payload
}

func (o *PcloudCloudinstancesImagesGetallOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Images)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudCloudinstancesImagesGetallBadRequest creates a PcloudCloudinstancesImagesGetallBadRequest with default headers values
func NewPcloudCloudinstancesImagesGetallBadRequest() *PcloudCloudinstancesImagesGetallBadRequest {
	return &PcloudCloudinstancesImagesGetallBadRequest{}
}

/* PcloudCloudinstancesImagesGetallBadRequest describes a response with status code 400, with default header values.

Bad Request
*/
type PcloudCloudinstancesImagesGetallBadRequest struct {
	Payload *models.Error
}

func (o *PcloudCloudinstancesImagesGetallBadRequest) Error() string {
	return fmt.Sprintf("[GET /pcloud/v1/cloud-instances/{cloud_instance_id}/images][%d] pcloudCloudinstancesImagesGetallBadRequest  %+v", 400, o.Payload)
}
func (o *PcloudCloudinstancesImagesGetallBadRequest) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudCloudinstancesImagesGetallBadRequest) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudCloudinstancesImagesGetallUnauthorized creates a PcloudCloudinstancesImagesGetallUnauthorized with default headers values
func NewPcloudCloudinstancesImagesGetallUnauthorized() *PcloudCloudinstancesImagesGetallUnauthorized {
	return &PcloudCloudinstancesImagesGetallUnauthorized{}
}

/* PcloudCloudinstancesImagesGetallUnauthorized describes a response with status code 401, with default header values.

Unauthorized
*/
type PcloudCloudinstancesImagesGetallUnauthorized struct {
	Payload *models.Error
}

func (o *PcloudCloudinstancesImagesGetallUnauthorized) Error() string {
	return fmt.Sprintf("[GET /pcloud/v1/cloud-instances/{cloud_instance_id}/images][%d] pcloudCloudinstancesImagesGetallUnauthorized  %+v", 401, o.Payload)
}
func (o *PcloudCloudinstancesImagesGetallUnauthorized) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudCloudinstancesImagesGetallUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudCloudinstancesImagesGetallNotFound creates a PcloudCloudinstancesImagesGetallNotFound with default headers values
func NewPcloudCloudinstancesImagesGetallNotFound() *PcloudCloudinstancesImagesGetallNotFound {
	return &PcloudCloudinstancesImagesGetallNotFound{}
}

/* PcloudCloudinstancesImagesGetallNotFound describes a response with status code 404, with default header values.

Not Found
*/
type PcloudCloudinstancesImagesGetallNotFound struct {
	Payload *models.Error
}

func (o *PcloudCloudinstancesImagesGetallNotFound) Error() string {
	return fmt.Sprintf("[GET /pcloud/v1/cloud-instances/{cloud_instance_id}/images][%d] pcloudCloudinstancesImagesGetallNotFound  %+v", 404, o.Payload)
}
func (o *PcloudCloudinstancesImagesGetallNotFound) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudCloudinstancesImagesGetallNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudCloudinstancesImagesGetallInternalServerError creates a PcloudCloudinstancesImagesGetallInternalServerError with default headers values
func NewPcloudCloudinstancesImagesGetallInternalServerError() *PcloudCloudinstancesImagesGetallInternalServerError {
	return &PcloudCloudinstancesImagesGetallInternalServerError{}
}

/* PcloudCloudinstancesImagesGetallInternalServerError describes a response with status code 500, with default header values.

Internal Server Error
*/
type PcloudCloudinstancesImagesGetallInternalServerError struct {
	Payload *models.Error
}

func (o *PcloudCloudinstancesImagesGetallInternalServerError) Error() string {
	return fmt.Sprintf("[GET /pcloud/v1/cloud-instances/{cloud_instance_id}/images][%d] pcloudCloudinstancesImagesGetallInternalServerError  %+v", 500, o.Payload)
}
func (o *PcloudCloudinstancesImagesGetallInternalServerError) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudCloudinstancesImagesGetallInternalServerError) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
