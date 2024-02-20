package filesystem

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetResourceVersion(obj runtime.Object) (uint64, error) {
	if obj == nil {
		return 0, nil
	}
	objMeta, err := meta.CommonAccessor(obj)
	if err != nil {
		return 0, err
	}
	return ParseResourceVersion(objMeta.GetResourceVersion())
}

func SetResourceVersion(obj runtime.Object, v uint64) error {
	if v <= 0 {
		return fmt.Errorf("resourceVersion must be positive: %d", v)
	}

	objMeta, err := meta.CommonAccessor(obj)
	if err != nil {
		return err
	}
	objMeta.SetResourceVersion(FormatResourceVersion(v))
	return nil
}

func ClearResourceVersion(obj runtime.Object) error {
	objMeta, err := meta.CommonAccessor(obj)
	if err != nil {
		return err
	}
	objMeta.SetResourceVersion("")
	return nil
}

func ParseResourceVersion(v string) (uint64, error) {
	if v == "" {
		return 0, nil
	}
	version, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func FormatResourceVersion(v uint64) string {
	return strconv.FormatUint(v, 10)
}
