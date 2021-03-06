// Code generated by go-swagger; DO NOT EDIT.

package authentication

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/IBM-Cloud/power-go-client/power/models"
)

// ServiceBrokerAuthTokenPostReader is a Reader for the ServiceBrokerAuthTokenPost structure.
type ServiceBrokerAuthTokenPostReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *ServiceBrokerAuthTokenPostReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewServiceBrokerAuthTokenPostOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := NewServiceBrokerAuthTokenPostBadRequest()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewServiceBrokerAuthTokenPostForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 429:
		result := NewServiceBrokerAuthTokenPostTooManyRequests()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 500:
		result := NewServiceBrokerAuthTokenPostInternalServerError()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewServiceBrokerAuthTokenPostOK creates a ServiceBrokerAuthTokenPostOK with default headers values
func NewServiceBrokerAuthTokenPostOK() *ServiceBrokerAuthTokenPostOK {
	return &ServiceBrokerAuthTokenPostOK{}
}

/* ServiceBrokerAuthTokenPostOK describes a response with status code 200, with default header values.

OK
*/
type ServiceBrokerAuthTokenPostOK struct {
	Payload *models.Token
}

func (o *ServiceBrokerAuthTokenPostOK) Error() string {
	return fmt.Sprintf("[POST /auth/v1/token][%d] serviceBrokerAuthTokenPostOK  %+v", 200, o.Payload)
}
func (o *ServiceBrokerAuthTokenPostOK) GetPayload() *models.Token {
	return o.Payload
}

func (o *ServiceBrokerAuthTokenPostOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Token)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceBrokerAuthTokenPostBadRequest creates a ServiceBrokerAuthTokenPostBadRequest with default headers values
func NewServiceBrokerAuthTokenPostBadRequest() *ServiceBrokerAuthTokenPostBadRequest {
	return &ServiceBrokerAuthTokenPostBadRequest{}
}

/* ServiceBrokerAuthTokenPostBadRequest describes a response with status code 400, with default header values.

Authorization pending
*/
type ServiceBrokerAuthTokenPostBadRequest struct {
	Payload *models.Error
}

func (o *ServiceBrokerAuthTokenPostBadRequest) Error() string {
	return fmt.Sprintf("[POST /auth/v1/token][%d] serviceBrokerAuthTokenPostBadRequest  %+v", 400, o.Payload)
}
func (o *ServiceBrokerAuthTokenPostBadRequest) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceBrokerAuthTokenPostBadRequest) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceBrokerAuthTokenPostForbidden creates a ServiceBrokerAuthTokenPostForbidden with default headers values
func NewServiceBrokerAuthTokenPostForbidden() *ServiceBrokerAuthTokenPostForbidden {
	return &ServiceBrokerAuthTokenPostForbidden{}
}

/* ServiceBrokerAuthTokenPostForbidden describes a response with status code 403, with default header values.

User refused grant
*/
type ServiceBrokerAuthTokenPostForbidden struct {
	Payload *models.Error
}

func (o *ServiceBrokerAuthTokenPostForbidden) Error() string {
	return fmt.Sprintf("[POST /auth/v1/token][%d] serviceBrokerAuthTokenPostForbidden  %+v", 403, o.Payload)
}
func (o *ServiceBrokerAuthTokenPostForbidden) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceBrokerAuthTokenPostForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceBrokerAuthTokenPostTooManyRequests creates a ServiceBrokerAuthTokenPostTooManyRequests with default headers values
func NewServiceBrokerAuthTokenPostTooManyRequests() *ServiceBrokerAuthTokenPostTooManyRequests {
	return &ServiceBrokerAuthTokenPostTooManyRequests{}
}

/* ServiceBrokerAuthTokenPostTooManyRequests describes a response with status code 429, with default header values.

Polling too frequently
*/
type ServiceBrokerAuthTokenPostTooManyRequests struct {
	Payload *models.Error
}

func (o *ServiceBrokerAuthTokenPostTooManyRequests) Error() string {
	return fmt.Sprintf("[POST /auth/v1/token][%d] serviceBrokerAuthTokenPostTooManyRequests  %+v", 429, o.Payload)
}
func (o *ServiceBrokerAuthTokenPostTooManyRequests) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceBrokerAuthTokenPostTooManyRequests) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewServiceBrokerAuthTokenPostInternalServerError creates a ServiceBrokerAuthTokenPostInternalServerError with default headers values
func NewServiceBrokerAuthTokenPostInternalServerError() *ServiceBrokerAuthTokenPostInternalServerError {
	return &ServiceBrokerAuthTokenPostInternalServerError{}
}

/* ServiceBrokerAuthTokenPostInternalServerError describes a response with status code 500, with default header values.

Internal Server Error
*/
type ServiceBrokerAuthTokenPostInternalServerError struct {
	Payload *models.Error
}

func (o *ServiceBrokerAuthTokenPostInternalServerError) Error() string {
	return fmt.Sprintf("[POST /auth/v1/token][%d] serviceBrokerAuthTokenPostInternalServerError  %+v", 500, o.Payload)
}
func (o *ServiceBrokerAuthTokenPostInternalServerError) GetPayload() *models.Error {
	return o.Payload
}

func (o *ServiceBrokerAuthTokenPostInternalServerError) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
