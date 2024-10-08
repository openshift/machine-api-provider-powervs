// Code generated by go-swagger; DO NOT EDIT.

package p_cloud_volumes

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/IBM-Cloud/power-go-client/power/models"
)

// PcloudV2VolumesGetallReader is a Reader for the PcloudV2VolumesGetall structure.
type PcloudV2VolumesGetallReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *PcloudV2VolumesGetallReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewPcloudV2VolumesGetallOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := NewPcloudV2VolumesGetallBadRequest()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 401:
		result := NewPcloudV2VolumesGetallUnauthorized()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewPcloudV2VolumesGetallForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewPcloudV2VolumesGetallNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 500:
		result := NewPcloudV2VolumesGetallInternalServerError()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes] pcloud.v2.volumes.getall", response, response.Code())
	}
}

// NewPcloudV2VolumesGetallOK creates a PcloudV2VolumesGetallOK with default headers values
func NewPcloudV2VolumesGetallOK() *PcloudV2VolumesGetallOK {
	return &PcloudV2VolumesGetallOK{}
}

/*
PcloudV2VolumesGetallOK describes a response with status code 200, with default header values.

OK
*/
type PcloudV2VolumesGetallOK struct {
	Payload *models.Volumes
}

// IsSuccess returns true when this pcloud v2 volumes getall o k response has a 2xx status code
func (o *PcloudV2VolumesGetallOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this pcloud v2 volumes getall o k response has a 3xx status code
func (o *PcloudV2VolumesGetallOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pcloud v2 volumes getall o k response has a 4xx status code
func (o *PcloudV2VolumesGetallOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this pcloud v2 volumes getall o k response has a 5xx status code
func (o *PcloudV2VolumesGetallOK) IsServerError() bool {
	return false
}

// IsCode returns true when this pcloud v2 volumes getall o k response a status code equal to that given
func (o *PcloudV2VolumesGetallOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the pcloud v2 volumes getall o k response
func (o *PcloudV2VolumesGetallOK) Code() int {
	return 200
}

func (o *PcloudV2VolumesGetallOK) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallOK %s", 200, payload)
}

func (o *PcloudV2VolumesGetallOK) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallOK %s", 200, payload)
}

func (o *PcloudV2VolumesGetallOK) GetPayload() *models.Volumes {
	return o.Payload
}

func (o *PcloudV2VolumesGetallOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Volumes)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudV2VolumesGetallBadRequest creates a PcloudV2VolumesGetallBadRequest with default headers values
func NewPcloudV2VolumesGetallBadRequest() *PcloudV2VolumesGetallBadRequest {
	return &PcloudV2VolumesGetallBadRequest{}
}

/*
PcloudV2VolumesGetallBadRequest describes a response with status code 400, with default header values.

Bad Request
*/
type PcloudV2VolumesGetallBadRequest struct {
	Payload *models.Error
}

// IsSuccess returns true when this pcloud v2 volumes getall bad request response has a 2xx status code
func (o *PcloudV2VolumesGetallBadRequest) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pcloud v2 volumes getall bad request response has a 3xx status code
func (o *PcloudV2VolumesGetallBadRequest) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pcloud v2 volumes getall bad request response has a 4xx status code
func (o *PcloudV2VolumesGetallBadRequest) IsClientError() bool {
	return true
}

// IsServerError returns true when this pcloud v2 volumes getall bad request response has a 5xx status code
func (o *PcloudV2VolumesGetallBadRequest) IsServerError() bool {
	return false
}

// IsCode returns true when this pcloud v2 volumes getall bad request response a status code equal to that given
func (o *PcloudV2VolumesGetallBadRequest) IsCode(code int) bool {
	return code == 400
}

// Code gets the status code for the pcloud v2 volumes getall bad request response
func (o *PcloudV2VolumesGetallBadRequest) Code() int {
	return 400
}

func (o *PcloudV2VolumesGetallBadRequest) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallBadRequest %s", 400, payload)
}

func (o *PcloudV2VolumesGetallBadRequest) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallBadRequest %s", 400, payload)
}

func (o *PcloudV2VolumesGetallBadRequest) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudV2VolumesGetallBadRequest) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudV2VolumesGetallUnauthorized creates a PcloudV2VolumesGetallUnauthorized with default headers values
func NewPcloudV2VolumesGetallUnauthorized() *PcloudV2VolumesGetallUnauthorized {
	return &PcloudV2VolumesGetallUnauthorized{}
}

/*
PcloudV2VolumesGetallUnauthorized describes a response with status code 401, with default header values.

Unauthorized
*/
type PcloudV2VolumesGetallUnauthorized struct {
	Payload *models.Error
}

// IsSuccess returns true when this pcloud v2 volumes getall unauthorized response has a 2xx status code
func (o *PcloudV2VolumesGetallUnauthorized) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pcloud v2 volumes getall unauthorized response has a 3xx status code
func (o *PcloudV2VolumesGetallUnauthorized) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pcloud v2 volumes getall unauthorized response has a 4xx status code
func (o *PcloudV2VolumesGetallUnauthorized) IsClientError() bool {
	return true
}

// IsServerError returns true when this pcloud v2 volumes getall unauthorized response has a 5xx status code
func (o *PcloudV2VolumesGetallUnauthorized) IsServerError() bool {
	return false
}

// IsCode returns true when this pcloud v2 volumes getall unauthorized response a status code equal to that given
func (o *PcloudV2VolumesGetallUnauthorized) IsCode(code int) bool {
	return code == 401
}

// Code gets the status code for the pcloud v2 volumes getall unauthorized response
func (o *PcloudV2VolumesGetallUnauthorized) Code() int {
	return 401
}

func (o *PcloudV2VolumesGetallUnauthorized) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallUnauthorized %s", 401, payload)
}

func (o *PcloudV2VolumesGetallUnauthorized) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallUnauthorized %s", 401, payload)
}

func (o *PcloudV2VolumesGetallUnauthorized) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudV2VolumesGetallUnauthorized) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudV2VolumesGetallForbidden creates a PcloudV2VolumesGetallForbidden with default headers values
func NewPcloudV2VolumesGetallForbidden() *PcloudV2VolumesGetallForbidden {
	return &PcloudV2VolumesGetallForbidden{}
}

/*
PcloudV2VolumesGetallForbidden describes a response with status code 403, with default header values.

Forbidden
*/
type PcloudV2VolumesGetallForbidden struct {
	Payload *models.Error
}

// IsSuccess returns true when this pcloud v2 volumes getall forbidden response has a 2xx status code
func (o *PcloudV2VolumesGetallForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pcloud v2 volumes getall forbidden response has a 3xx status code
func (o *PcloudV2VolumesGetallForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pcloud v2 volumes getall forbidden response has a 4xx status code
func (o *PcloudV2VolumesGetallForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this pcloud v2 volumes getall forbidden response has a 5xx status code
func (o *PcloudV2VolumesGetallForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this pcloud v2 volumes getall forbidden response a status code equal to that given
func (o *PcloudV2VolumesGetallForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the pcloud v2 volumes getall forbidden response
func (o *PcloudV2VolumesGetallForbidden) Code() int {
	return 403
}

func (o *PcloudV2VolumesGetallForbidden) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallForbidden %s", 403, payload)
}

func (o *PcloudV2VolumesGetallForbidden) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallForbidden %s", 403, payload)
}

func (o *PcloudV2VolumesGetallForbidden) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudV2VolumesGetallForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudV2VolumesGetallNotFound creates a PcloudV2VolumesGetallNotFound with default headers values
func NewPcloudV2VolumesGetallNotFound() *PcloudV2VolumesGetallNotFound {
	return &PcloudV2VolumesGetallNotFound{}
}

/*
PcloudV2VolumesGetallNotFound describes a response with status code 404, with default header values.

Not Found
*/
type PcloudV2VolumesGetallNotFound struct {
	Payload *models.Error
}

// IsSuccess returns true when this pcloud v2 volumes getall not found response has a 2xx status code
func (o *PcloudV2VolumesGetallNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pcloud v2 volumes getall not found response has a 3xx status code
func (o *PcloudV2VolumesGetallNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pcloud v2 volumes getall not found response has a 4xx status code
func (o *PcloudV2VolumesGetallNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this pcloud v2 volumes getall not found response has a 5xx status code
func (o *PcloudV2VolumesGetallNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this pcloud v2 volumes getall not found response a status code equal to that given
func (o *PcloudV2VolumesGetallNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the pcloud v2 volumes getall not found response
func (o *PcloudV2VolumesGetallNotFound) Code() int {
	return 404
}

func (o *PcloudV2VolumesGetallNotFound) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallNotFound %s", 404, payload)
}

func (o *PcloudV2VolumesGetallNotFound) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallNotFound %s", 404, payload)
}

func (o *PcloudV2VolumesGetallNotFound) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudV2VolumesGetallNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewPcloudV2VolumesGetallInternalServerError creates a PcloudV2VolumesGetallInternalServerError with default headers values
func NewPcloudV2VolumesGetallInternalServerError() *PcloudV2VolumesGetallInternalServerError {
	return &PcloudV2VolumesGetallInternalServerError{}
}

/*
PcloudV2VolumesGetallInternalServerError describes a response with status code 500, with default header values.

Internal Server Error
*/
type PcloudV2VolumesGetallInternalServerError struct {
	Payload *models.Error
}

// IsSuccess returns true when this pcloud v2 volumes getall internal server error response has a 2xx status code
func (o *PcloudV2VolumesGetallInternalServerError) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this pcloud v2 volumes getall internal server error response has a 3xx status code
func (o *PcloudV2VolumesGetallInternalServerError) IsRedirect() bool {
	return false
}

// IsClientError returns true when this pcloud v2 volumes getall internal server error response has a 4xx status code
func (o *PcloudV2VolumesGetallInternalServerError) IsClientError() bool {
	return false
}

// IsServerError returns true when this pcloud v2 volumes getall internal server error response has a 5xx status code
func (o *PcloudV2VolumesGetallInternalServerError) IsServerError() bool {
	return true
}

// IsCode returns true when this pcloud v2 volumes getall internal server error response a status code equal to that given
func (o *PcloudV2VolumesGetallInternalServerError) IsCode(code int) bool {
	return code == 500
}

// Code gets the status code for the pcloud v2 volumes getall internal server error response
func (o *PcloudV2VolumesGetallInternalServerError) Code() int {
	return 500
}

func (o *PcloudV2VolumesGetallInternalServerError) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallInternalServerError %s", 500, payload)
}

func (o *PcloudV2VolumesGetallInternalServerError) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /pcloud/v2/cloud-instances/{cloud_instance_id}/volumes][%d] pcloudV2VolumesGetallInternalServerError %s", 500, payload)
}

func (o *PcloudV2VolumesGetallInternalServerError) GetPayload() *models.Error {
	return o.Payload
}

func (o *PcloudV2VolumesGetallInternalServerError) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
