// Code generated by go-swagger; DO NOT EDIT.

package service_instances

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/IBM-Cloud/power-go-client/power/models"
)

// ServiceInstanceGetReader is a Reader for the ServiceInstanceGet structure.
type ServiceInstanceGetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *ServiceInstanceGetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewServiceInstanceGetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 404:
		result := NewServiceInstanceGetNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewServiceInstanceGetOK creates a ServiceInstanceGetOK with default headers values
func NewServiceInstanceGetOK() *ServiceInstanceGetOK {
	return &ServiceInstanceGetOK{}
}

/* ServiceInstanceGetOK describes a response with status code 200, with default header values.

OK
*/
type ServiceInstanceGetOK struct {
	Payload *models.ServiceInstanceResource
}

func (o *ServiceInstanceGetOK) Error() string {
	return fmt.Sprintf("[GET /v2/service_instances/{instance_id}][%d] serviceInstanceGetOK  %+v", 200, o.Payload)
}
func (o *ServiceInstanceGetOK) GetPayload() *models.ServiceInstanceResource {
	return o.Payload
}

func (o *ServiceInstanceGetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ServiceInstanceResource)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceInstanceGetNotFound creates a ServiceInstanceGetNotFound with default headers values
func NewServiceInstanceGetNotFound() *ServiceInstanceGetNotFound {
	return &ServiceInstanceGetNotFound{}
}

/* ServiceInstanceGetNotFound describes a response with status code 404, with default header values.

Not Found
*/
type ServiceInstanceGetNotFound struct {
	Payload *models.Error
}

func (o *ServiceInstanceGetNotFound) Error() string {
	return fmt.Sprintf("[GET /v2/service_instances/{instance_id}][%d] serviceInstanceGetNotFound  %+v", 404, o.Payload)
}
func (o *ServiceInstanceGetNotFound) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceInstanceGetNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
