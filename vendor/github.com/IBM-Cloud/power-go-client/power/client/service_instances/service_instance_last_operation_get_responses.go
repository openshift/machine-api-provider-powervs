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

// ServiceInstanceLastOperationGetReader is a Reader for the ServiceInstanceLastOperationGet structure.
type ServiceInstanceLastOperationGetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *ServiceInstanceLastOperationGetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewServiceInstanceLastOperationGetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := NewServiceInstanceLastOperationGetBadRequest()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 410:
		result := NewServiceInstanceLastOperationGetGone()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewServiceInstanceLastOperationGetOK creates a ServiceInstanceLastOperationGetOK with default headers values
func NewServiceInstanceLastOperationGetOK() *ServiceInstanceLastOperationGetOK {
	return &ServiceInstanceLastOperationGetOK{}
}

/* ServiceInstanceLastOperationGetOK describes a response with status code 200, with default header values.

OK
*/
type ServiceInstanceLastOperationGetOK struct {
	Payload *models.LastOperationResource
}

func (o *ServiceInstanceLastOperationGetOK) Error() string {
	return fmt.Sprintf("[GET /v2/service_instances/{instance_id}/last_operation][%d] serviceInstanceLastOperationGetOK  %+v", 200, o.Payload)
}
func (o *ServiceInstanceLastOperationGetOK) GetPayload() *models.LastOperationResource {
	return o.Payload
}

func (o *ServiceInstanceLastOperationGetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.LastOperationResource)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceInstanceLastOperationGetBadRequest creates a ServiceInstanceLastOperationGetBadRequest with default headers values
func NewServiceInstanceLastOperationGetBadRequest() *ServiceInstanceLastOperationGetBadRequest {
	return &ServiceInstanceLastOperationGetBadRequest{}
}

/* ServiceInstanceLastOperationGetBadRequest describes a response with status code 400, with default header values.

Bad Request
*/
type ServiceInstanceLastOperationGetBadRequest struct {
	Payload *models.Error
}

func (o *ServiceInstanceLastOperationGetBadRequest) Error() string {
	return fmt.Sprintf("[GET /v2/service_instances/{instance_id}/last_operation][%d] serviceInstanceLastOperationGetBadRequest  %+v", 400, o.Payload)
}
func (o *ServiceInstanceLastOperationGetBadRequest) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceInstanceLastOperationGetBadRequest) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceInstanceLastOperationGetGone creates a ServiceInstanceLastOperationGetGone with default headers values
func NewServiceInstanceLastOperationGetGone() *ServiceInstanceLastOperationGetGone {
	return &ServiceInstanceLastOperationGetGone{}
}

/* ServiceInstanceLastOperationGetGone describes a response with status code 410, with default header values.

Gone
*/
type ServiceInstanceLastOperationGetGone struct {
	Payload *models.Error
}

func (o *ServiceInstanceLastOperationGetGone) Error() string {
	return fmt.Sprintf("[GET /v2/service_instances/{instance_id}/last_operation][%d] serviceInstanceLastOperationGetGone  %+v", 410, o.Payload)
}
func (o *ServiceInstanceLastOperationGetGone) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceInstanceLastOperationGetGone) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
