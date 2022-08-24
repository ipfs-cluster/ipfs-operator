package utils

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CreateOrPatchTrackedObjects Goes through the map of tracked objects and attempts to
// apply the ctrl.createOrPatch function to each one. This function will return a
// boolean indicating whether or not the requeue should be set to true.
func CreateOrPatchTrackedObjects(
	ctx context.Context,
	trackedObjects map[client.Object]controllerutil.MutateFn,
	client client.Client,
	log logr.Logger,
) bool {
	var requeue bool
	var err error
	for obj, mut := range trackedObjects {
		var result controllerutil.OperationResult
		kind := obj.GetObjectKind().GroupVersionKind()
		name := obj.GetName()
		result, err = controllerutil.CreateOrPatch(ctx, client, obj, mut)
		if err != nil {
			log.Error(err, "error creating object", "objname", name, "objKind", kind.Kind, "result", result)
			requeue = true
		} else {
			log.Info("object changed", "objName", name, "objKind", kind.Kind, "result", result)
		}
	}
	return requeue
}

// ErrFunc Returns a function which returns the provided error when called.
func ErrFunc(err error) controllerutil.MutateFn {
	return func() error {
		return err
	}
}
