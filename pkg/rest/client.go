/**
 * @license
 * Copyright 2020 Dynatrace LLC
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rest

import (
	"errors"
	"net/http"

	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/api"
)

//go:generate mockgen -source=client.go -destination=client_mock.go -package=rest DynatraceClient

// DynatraceClient provides the functionality for performing basic CRUD operations on any Dynatrace API
// supported by monaco.
// It encapsulates the configuration-specific inconsistencies of certain APIs in one place to provide
// a common interface to work with. After all: A user of DynatraceClient shouldn't care about the
// implementation details of each individual Dynatrace API.
// Its design is intentionally not dependant on the Config and Environment interfaces included in monaco.
// This makes sure, that DynatraceClient can be used as a base for future tooling, which relies on
// a standardized way to access Dynatrace APIs.
type DynatraceClient interface {

	// List lists the available configs for an API.
	// It calls the underlying GET endpoint of the API. E.g. for alerting profiles this would be:
	//    GET <environment-url>/api/config/v1/alertingProfiles
	// The result is expressed using a list of Value (id and name tuples).
	List(api api.Api) (values []api.Value, err error)

	// ReadByName reads a Dynatrace config identified by name from the given API.
	// It calls the underlying GET endpoints for the API. E.g. for alerting profiles this would be:
	//    GET <environment-url>/api/config/v1/alertingProfiles ... to get the id of the existing alerting profile
	//    GET <environment-url>/api/config/v1/alertingProfiles/<id> ... to get the alerting profile
	ReadByName(api api.Api, name string) (json []byte, err error)

	// ReadById reads a Dynatrace config identified by id from the given API.
	// It calls the underlying GET endpoint for the API. E.g. for alerting profiles this would be:
	//    GET <environment-url>/api/config/v1/alertingProfiles/<id> ... to get the alerting profile
	ReadById(api api.Api, name string) (json []byte, err error)

	// Upsert creates a given Dynatrace config it it doesn't exists and updates it otherwise using its name
	// It calls the underlying GET, POST, and PUT endpoints for the API. E.g. for alerting profiles this would be:
	//    GET <environment-url>/api/config/v1/alertingProfiles ... to check if the config is already available
	//    POST <environment-url>/api/config/v1/alertingProfiles ... afterwards, if the config is not yet available
	//    PUT <environment-url>/api/config/v1/alertingProfiles/<id> ... instead of POST, if the config is already available
	UpsertByName(api api.Api, name, json string) (entity api.DynatraceEntity, err error)

	// Delete removed a given config for a given API using its name.
	// It calls the underlying GET and DELETE endpoints for the API. E.g. for alerting profiles this would be:
	//    GET <environment-url>/api/config/v1/alertingProfiles ... to get the id of the existing config
	//    DELETE <environment-url>/api/config/v1/alertingProfiles/<id> ... to delete the config
	DeleteByName(api api.Api, name string) error

	// ExistsByName checks if a config with the given name exists for the given API.
	// It cally the underlying GET endpoint for the API. E.g. for alerting profiles this would be:
	//    GET <environment-url>/api/config/v1/alertingProfiles
	ExistsByName(api api.Api, name string) (exists bool, id string, err error)
}

type dynatraceClientImpl struct {
	environmentUrl string
	token          string
	client         *http.Client
}

// NewDynatraceClient creates a new DynatraceClient
func NewDynatraceClient(environmentUrl, token string) DynatraceClient {

	return &dynatraceClientImpl{
		environmentUrl: environmentUrl,
		token:          token,
		client:         &http.Client{},
	}
}
func (d *dynatraceClientImpl) List(api api.Api) (values []api.Value, err error) {

	url := api.GetUrlFromEnvironmentUrl(d.environmentUrl)
	_, values, err = getExistingValuesFromEndpoint(d.client, api.GetId(), url, d.token)
	return values, err
}

func (d *dynatraceClientImpl) ReadByName(api api.Api, name string) (json []byte, err error) {

	exists, id, err := d.ExistsByName(api, name)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errors.New("404 - no config found with name " + name)
	}

	return d.ReadById(api, id)
}

func (d *dynatraceClientImpl) ReadById(api api.Api, id string) (json []byte, err error) {

	url := api.GetUrlFromEnvironmentUrl(d.environmentUrl) + "/" + id
	response := get(d.client, url, d.token)
	return response.Body, nil
}

func (d *dynatraceClientImpl) DeleteByName(api api.Api, name string) error {

	return deleteDynatraceObject(d.client, api.GetId(), name, api.GetUrlFromEnvironmentUrl(d.environmentUrl), d.token)
}

func (d *dynatraceClientImpl) ExistsByName(api api.Api, name string) (exists bool, id string, err error) {

	_, existingObjectId, err := getObjectIdIfAlreadyExists(d.client, api.GetId(), api.GetUrlFromEnvironmentUrl(d.environmentUrl), name, d.token)
	return existingObjectId != "", existingObjectId, err
}

func (d *dynatraceClientImpl) UpsertByName(api api.Api, json, name string) (entity api.DynatraceEntity, err error) {

	url := api.GetUrlFromEnvironmentUrl(d.environmentUrl)

	if api.GetId() == "extension" {
		return uploadExtension(d.client, url, name, json, d.token)
	}
	return upsertDynatraceObject(d.client, url, name, api.GetId(), json, d.token)
}
