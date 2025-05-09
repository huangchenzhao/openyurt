/*
Copyright 2024 The OpenYurt Authors.

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

package testing

import (
	"net/http"

	"github.com/openyurtio/openyurt/pkg/yurthub/filter"
)

type EmptyFilterManager struct {
}

func (fm *EmptyFilterManager) FindResponseFilter(req *http.Request) (filter.ResponseFilter, bool) {
	return nil, false
}

func (fm *EmptyFilterManager) FindObjectFilter(req *http.Request) (filter.ObjectFilter, bool) {
	return nil, false
}

func (fm *EmptyFilterManager) HasSynced() bool {
	return true
}

type FakeEndpointSliceFilter struct {
	NodeName string
}

func (fm *FakeEndpointSliceFilter) FindResponseFilter(req *http.Request) (filter.ResponseFilter, bool) {
	return nil, false
}

func (fm *FakeEndpointSliceFilter) FindObjectFilter(req *http.Request) (filter.ObjectFilter, bool) {
	return &IgnoreEndpointslicesWithNodeName{
		fm.NodeName,
	}, true
}

func (fm *FakeEndpointSliceFilter) HasSynced() bool {
	return true
}
