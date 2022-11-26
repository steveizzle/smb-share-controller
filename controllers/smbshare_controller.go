/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sharev1beta1 "github.com/steveizzle/smb-share-controller/api/v1beta1"
)

// SmbShareReconciler reconciles a SmbShare object
type SmbShareReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=share.k8s.hirnkastl.com,resources=smbshares,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=share.k8s.hirnkastl.com,resources=smbshares/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=share.k8s.hirnkastl.com,resources=smbshares/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumes/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SmbShareReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logging := log.FromContext(ctx)

	logging.Info(fmt.Sprintf("Got Object, Name: %s; Namespace: %s", req.Name, req.Namespace))

	pvName := fmt.Sprintf("smb-%s-%s", req.Namespace, req.Name)

	var (
		share sharev1beta1.SmbShare
		pv    corev1.PersistentVolume
		pvc   corev1.PersistentVolumeClaim
	)

	smberr := r.Get(ctx, req.NamespacedName, &share)
	if smberr != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(smberr)
	}

	// finalizer part
	finalizerName := "share.k8s.hirnkastl.com/finalizer"

	if share.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(&share, finalizerName) {
			controllerutil.AddFinalizer(&share, finalizerName)
			if err := r.Update(ctx, &share); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(&share, finalizerName) {
			// Get the pvName
			if err := r.Get(ctx, types.NamespacedName{
				Name:      pvName,
				Namespace: req.Namespace,
			}, &pv); err == nil {
				// delete PV
				if delerr := r.Delete(ctx, &pv); delerr != nil {
					logging.Error(delerr, "Error when deleting PV %s", pv.ObjectMeta.Name)
					// return with error so it can be retried
					return ctrl.Result{}, delerr
				}
			}
			logging.Info(fmt.Sprintf("PV %s deleted", pv.ObjectMeta.Name))
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&share, finalizerName)
			if err := r.Update(ctx, &share); err != nil {
				return ctrl.Result{}, err
			}
			// Stop reconciliation as the item is being deleted
			return ctrl.Result{}, nil
		}
	}

	// It is a newly created smbshare
	newPv := createPv(pvName, req.Namespace, share.Spec.SecretName, share.Spec.Path, share.Spec.MountOptions, share.ObjectMeta.Name)

	// Getting pv
	if err := r.Get(ctx, types.NamespacedName{
		Name:      pvName,
		Namespace: req.Namespace,
	}, &pv); err != nil {
		// if not found and share object exists, create pv
		if apierrors.IsNotFound(err) && smberr == nil {
			logging.Info(err.Error())
			if pvCreateErr := r.Client.Create(ctx, newPv); pvCreateErr != nil {
				logging.Error(pvCreateErr, fmt.Sprintf("Failed to create PV : %s", pvName))
			} else {
				logging.Info(fmt.Sprintf("Created PV: %s", pvName))
				share.Status.PvName = pvName
				// Update status field
				if err := r.Status().Update(ctx, &share); err != nil {
					logging.Error(err, fmt.Sprintf("Error Updating status of smbshare %v", &share.ObjectMeta.Name))
					return ctrl.Result{}, err
				}
			}
		} else {
			logging.Error(client.IgnoreNotFound(err), "pv error")
		} // apply could be later added here to e..g change mount_options
	}

	// INFO: used to set owner, but didnt work because cluster objects cant be owned by namespaced objects
	// finalizer is used instead
	// if err := controllerutil.SetControllerReference(share, pv, r.Scheme); err != nil {
	// 	logging.Error(err, "Failed to set reference")
	// }

	// creating the pvc
	newPvc := createPvc(req.Name, req.Namespace, pvName)

	// Getting pvc
	if err := r.Get(ctx, types.NamespacedName{
		Name:      newPvc.Name,
		Namespace: req.Namespace,
	}, &pvc); err != nil {
		// if not found and share object exists, create pv
		if apierrors.IsNotFound(err) && smberr == nil {
			// set owner of pvc to smbshare
			if referenceErr := controllerutil.SetControllerReference(&share, newPvc, r.Scheme); referenceErr != nil {
				logging.Error(referenceErr, fmt.Sprintf("Failed to set reference for PVC: %s", newPvc.Name))
			} else {
				logging.Info("Reference set")
			}
			if pvcCreateErr := r.Client.Create(ctx, newPvc); pvcCreateErr != nil {
				logging.Error(pvcCreateErr, fmt.Sprintf("Failed to create PVC : %s", newPvc.Name))
			} else {
				logging.Info(fmt.Sprintf("Created PVC: %s", newPvc.Name))
				share.Status.PvcName = newPvc.Name
				// Update status field
				if err := r.Status().Update(ctx, &share); err != nil {
					logging.Error(err, fmt.Sprintf("Error Updating status of smbshare %v", &share.ObjectMeta.Name))
					return ctrl.Result{}, err
				}
			}
		}
	}

	// end reconciliation
	return ctrl.Result{}, nil
}

func updateStatus(ctx context.Context, share sharev1beta1.SmbShare) {

}

// helper function to create pv
func createPvc(name, namespace, pvName string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			VolumeName:       pvName,
			StorageClassName: new(string), // must be empty string
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("100Gi"),
				},
			},
		},
	}
}

// helper function to create pvc
func createPv(pvName, secretNamespace, secretName, path string, mountOptions []string, shareName string) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvName,
			Labels: map[string]string{
				"smb-controller": shareName,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				"storage": resource.MustParse("100Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			// INFO: Reclaim Policy unfortunately dont work with statically configured PVs
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			MountOptions:                  mountOptions,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "smb.csi.k8s.io",
					ReadOnly:     false,
					VolumeHandle: pvName,
					VolumeAttributes: map[string]string{
						"source": path,
					},
					NodeStageSecretRef: &corev1.SecretReference{
						Name:      secretName,
						Namespace: secretNamespace,
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SmbShareReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sharev1beta1.SmbShare{}).
		// reconciliation for pvc, but not inevitable necesseray
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
