// Code generated by go-swagger; DO NOT EDIT.

package open_stacks

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/IBM-Cloud/power-go-client/power/models"
)

// ServiceBrokerOpenstacksServersGetReader is a Reader for the ServiceBrokerOpenstacksServersGet structure.
type ServiceBrokerOpenstacksServersGetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *ServiceBrokerOpenstacksServersGetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewServiceBrokerOpenstacksServersGetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := NewServiceBrokerOpenstacksServersGetBadRequest()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewServiceBrokerOpenstacksServersGetNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 500:
		result := NewServiceBrokerOpenstacksServersGetInternalServerError()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewServiceBrokerOpenstacksServersGetOK creates a ServiceBrokerOpenstacksServersGetOK with default headers values
func NewServiceBrokerOpenstacksServersGetOK() *ServiceBrokerOpenstacksServersGetOK {
	return &ServiceBrokerOpenstacksServersGetOK{}
}

/* ServiceBrokerOpenstacksServersGetOK describes a response with status code 200, with default header values.

OK
*/
type ServiceBrokerOpenstacksServersGetOK struct {
	Payload *models.HostPVMInstance
}

func (o *ServiceBrokerOpenstacksServersGetOK) Error() string {
	return fmt.Sprintf("[GET /broker/v1/openstacks/{openstack_id}/servers/{pvm_instance_id}][%d] serviceBrokerOpenstacksServersGetOK  %+v", 200, o.Payload)
}
func (o *ServiceBrokerOpenstacksServersGetOK) GetPayload() *models.HostPVMInstance {
	return o.Payload
}

func (o *ServiceBrokerOpenstacksServersGetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.HostPVMInstance)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceBrokerOpenstacksServersGetBadRequest creates a ServiceBrokerOpenstacksServersGetBadRequest with default headers values
func NewServiceBrokerOpenstacksServersGetBadRequest() *ServiceBrokerOpenstacksServersGetBadRequest {
	return &ServiceBrokerOpenstacksServersGetBadRequest{}
}

/* ServiceBrokerOpenstacksServersGetBadRequest describes a response with status code 400, with default header values.

Bad Request
*/
type ServiceBrokerOpenstacksServersGetBadRequest struct {
	Payload *models.Error
}

func (o *ServiceBrokerOpenstacksServersGetBadRequest) Error() string {
	return fmt.Sprintf("[GET /broker/v1/openstacks/{openstack_id}/servers/{pvm_instance_id}][%d] serviceBrokerOpenstacksServersGetBadRequest  %+v", 400, o.Payload)
}
func (o *ServiceBrokerOpenstacksServersGetBadRequest) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceBrokerOpenstacksServersGetBadRequest) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceBrokerOpenstacksServersGetNotFound creates a ServiceBrokerOpenstacksServersGetNotFound with default headers values
func NewServiceBrokerOpenstacksServersGetNotFound() *ServiceBrokerOpenstacksServersGetNotFound {
	return &ServiceBrokerOpenstacksServersGetNotFound{}
}

/* ServiceBrokerOpenstacksServersGetNotFound describes a response with status code 404, with default header values.

Not Found
*/
type ServiceBrokerOpenstacksServersGetNotFound struct {
	Payload *models.Error
}

func (o *ServiceBrokerOpenstacksServersGetNotFound) Error() string {
	return fmt.Sprintf("[GET /broker/v1/openstacks/{openstack_id}/servers/{pvm_instance_id}][%d] serviceBrokerOpenstacksServersGetNotFound  %+v", 404, o.Payload)
}
func (o *ServiceBrokerOpenstacksServersGetNotFound) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceBrokerOpenstacksServersGetNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceBrokerOpenstacksServersGetInternalServerError creates a ServiceBrokerOpenstacksServersGetInternalServerError with default headers values
func NewServiceBrokerOpenstacksServersGetInternalServerError() *ServiceBrokerOpenstacksServersGetInternalServerError {
	return &ServiceBrokerOpenstacksServersGetInternalServerError{}
}

/* ServiceBrokerOpenstacksServersGetInternalServerError describes a response with status code 500, with default header values.

Internal Server Error
*/
type ServiceBrokerOpenstacksServersGetInternalServerError struct {
	Payload *models.Error
}

func (o *ServiceBrokerOpenstacksServersGetInternalServerError) Error() string {
	return fmt.Sprintf("[GET /broker/v1/openstacks/{openstack_id}/servers/{pvm_instance_id}][%d] serviceBrokerOpenstacksServersGetInternalServerError  %+v", 500, o.Payload)
}
func (o *ServiceBrokerOpenstacksServersGetInternalServerError) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceBrokerOpenstacksServersGetInternalServerError) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
