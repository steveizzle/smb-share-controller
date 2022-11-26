package controllers

import (
	"context"
	"fmt"

	// "reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	sharev1beta1 "github.com/steveizzle/smb-share-controller/api/v1beta1"
)

var _ = Describe("Smb Controller", func() {
	const (
		SmbName       = "test-share"
		SmbNamespace  = "default"
		SmbPath       = "//testserver/a/b/c"
		SmbSecretName = "test-secret"
		SmbKind       = "SmbShare"
		SmbAPI        = "share.k8s.hirnkastl.com"
		finalizer     = "share.k8s.hirnkastl.com/finalizer"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	var mountOptions = []string{"filemode=777"}

	Context("Setting up a Smb Share should Succeed", func() {
		It("Should set up a new PVC and PV", func() {
			By("Creating a new Share")
			ctx := context.Background()
			share := &sharev1beta1.SmbShare{
				TypeMeta: metav1.TypeMeta{
					APIVersion: SmbAPI,
					Kind:       SmbKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      SmbName,
					Namespace: SmbNamespace,
				},
				Spec: sharev1beta1.SmbShareSpec{
					Path:         SmbPath,
					SecretName:   SmbSecretName,
					MountOptions: mountOptions,
				},
			}
			Expect(k8sClient.Create(ctx, share)).Should(Succeed())

			shareLookupKey := types.NamespacedName{Name: share.Name, Namespace: share.Namespace}
			createdShare := &sharev1beta1.SmbShare{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, shareLookupKey, createdShare)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			pvName := fmt.Sprintf("smb-%s-%s", share.Namespace, share.Name)
			pvLookupKey := types.NamespacedName{Name: pvName, Namespace: share.Namespace}

			pvcName := share.Name
			pvcLookupKey := types.NamespacedName{Name: pvcName, Namespace: share.Namespace}

			/*
				PV should be found
			*/

			pv := &corev1.PersistentVolume{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, pvLookupKey, pv)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			/*
				PV Spec should contain expected values
			*/

			Expect(pv.Spec.AccessModes).Should(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))
			Expect(pv.Spec.PersistentVolumeSource.CSI.VolumeHandle).Should(Equal(pvName))
			Expect(pv.Spec.PersistentVolumeSource.CSI.VolumeAttributes).Should(Equal(map[string]string{
				"source": SmbPath,
			}))
			Expect(*pv.Spec.PersistentVolumeSource.CSI.NodeStageSecretRef).Should(BeEquivalentTo(corev1.SecretReference{
				Name:      SmbSecretName,
				Namespace: SmbNamespace,
			}))
			Expect(pv.Spec.MountOptions).Should(Equal(mountOptions))

			/*
				PVC should be found
			*/
			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, pvcLookupKey, pvc)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			/*
				PVC Spec should contain expected values
			*/

			Expect(pvc.Spec.VolumeName).Should(Equal(pvName))
			Expect(pvc.Spec.StorageClassName).Should(Equal(new(string)))
			Expect(pvc.ObjectMeta.Name).Should(Equal(pvcName))

			/*
				Edits should not have an effect
			*/

			/*
				Both should get deleted when SmbShare is deleted. In Envtest no controller is running for volumes.
				So OwnerRefernece and Finalizers gets checked
			*/

			Expect(k8sClient.Delete(ctx, createdShare)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, shareLookupKey, createdShare)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeFalse())

			Expect(createdShare.Finalizers).Should(Equal([]string{finalizer}))
			Expect(createdShare.Status.PvName).Should(Equal(pvName))
			Expect(createdShare.Status.PvcName).Should(Equal(pvcName))
			Expect(pvc.OwnerReferences[len(pvc.OwnerReferences)-1].Name).Should(Equal(SmbName))
			Expect(pvc.OwnerReferences[len(pvc.OwnerReferences)-1].Kind).Should(Equal(SmbKind))

		})
	})
})
