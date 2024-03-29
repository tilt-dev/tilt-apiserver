/*
Copyright 2017 The Kubernetes Authors.

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

package apiserver

import (
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	pkgserver "k8s.io/apiserver/pkg/server"
)

type StorageProvider func(s *runtime.Scheme, g genericregistry.RESTOptionsGetter) (rest.Storage, error)

func buildAPIGroupInfos(scheme *runtime.Scheme,
	codecs serializer.CodecFactory,
	apiMap map[schema.GroupVersionResource]StorageProvider,
	g genericregistry.RESTOptionsGetter,
	parameterCodec runtime.ParameterCodec) ([]*pkgserver.APIGroupInfo, error) {
	resourcesByGroupVersion := make(map[schema.GroupVersion]sets.String)
	groups := sets.NewString()
	if parameterCodec == nil {
		parameterCodec = metav1.ParameterCodec
	}
	for gvr := range apiMap {
		groups.Insert(gvr.Group)
		if resourcesByGroupVersion[gvr.GroupVersion()] == nil {
			resourcesByGroupVersion[gvr.GroupVersion()] = sets.NewString()
		}
		resourcesByGroupVersion[gvr.GroupVersion()].Insert(gvr.Resource)
	}
	apiGroups := []*pkgserver.APIGroupInfo{}
	for _, group := range groups.List() {
		apis := map[string]map[string]rest.Storage{}
		for gvr, storageProviderFunc := range apiMap {
			if gvr.Group == group {
				if _, found := apis[gvr.Version]; !found {
					apis[gvr.Version] = map[string]rest.Storage{}
				}
				storage, err := storageProviderFunc(scheme, g)
				if err != nil {
					return nil, err
				}
				apis[gvr.Version][gvr.Resource] = storage
				if _, ok := storage.(resourcestrategy.Defaulter); ok {
					if obj, ok := storage.(runtime.Object); ok {
						scheme.AddTypeDefaultingFunc(obj, func(obj interface{}) {
							obj.(resourcestrategy.Defaulter).Default()
						})
					}
				}
			}
		}
		apiGroupInfo := pkgserver.NewDefaultAPIGroupInfo(group, scheme, parameterCodec, codecs)
		apiGroupInfo.VersionedResourcesStorageMap = apis
		apiGroups = append(apiGroups, &apiGroupInfo)
	}
	return apiGroups, nil
}
