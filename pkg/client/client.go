/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	bluemixmodels "github.com/IBM-Cloud/bluemix-go/models"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

//go:generate go run ../../vendor/github.com/golang/mock/mockgen -source=./client.go -destination=./mock/client_generated.go -package=mock

// Client is a wrapper object for actual PowerVS SDK clients to allow for easier testing.
type Client interface {
	CreateInstance(createParams *models.PVMInstanceCreate) (*models.PVMInstanceList, error)
	GetInstance(id string) (*models.PVMInstance, error)
	GetInstanceByName(name string) (*models.PVMInstance, error)
	GetInstances() (*models.PVMInstances, error)
	DeleteInstance(id string) error
	GetImages() (*models.Images, error)
	GetNetworks() (*models.Networks, error)
	GetCloudServiceInstances() ([]bluemixmodels.ServiceInstanceV2, error)
}
