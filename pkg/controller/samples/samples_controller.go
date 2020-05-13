package samples

import (
	"context"
	"encoding/json"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
	"time"

	olmv1alpha1 "github.com/gabemontero/openshift-samples-olm-operator/pkg/apis/olm/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	imageapiv1 "github.com/openshift/api/image/v1"
	templatev1 "github.com/openshift/api/template/v1"
	imageset "github.com/openshift/client-go/image/clientset/versioned"
	templateset "github.com/openshift/client-go/template/clientset/versioned"
)

var log = logf.Log.WithName("controller_samples")

// Add creates a new Samples Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSamples{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("samples-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Samples
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.Samples{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Samples
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &olmv1alpha1.Samples{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSamples implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSamples{}

const ns = "openshift"

// ReconcileSamples reconciles a Samples object
type ReconcileSamples struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Samples object and makes changes based on the state read
// and what is in the Samples.Spec
func (r *ReconcileSamples) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Samples")

	// relist
	interval := 15 * time.Minute

	// Fetch the Samples instance
	instance := &olmv1alpha1.Samples{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("No config object found, will relist in %s", interval.String())
			return reconcile.Result{Requeue: true, RequeueAfter: interval}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Info("Problem reading config object: %s; will relist in %s", err.Error(), interval.String())
		return reconcile.Result{Requeue: true, RequeueAfter: interval}, err
	}

	if instance.Spec.RelistInterval > 0 {
		interval = time.Duration(instance.Spec.RelistInterval) * time.Minute
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		return reconcile.Result{}, err
	}

	imageclient, err := imageset.NewForConfig(cfg)
	if err != nil {
		reqLogger.Info("Error creating image client %s", err.Error())
		return reconcile.Result{}, err
	}
	templateclient, err := templateset.NewForConfig(cfg)
	if err != nil {
		reqLogger.Info("Error creating template client %s", err.Error())
		return reconcile.Result{}, err
	}
	httpclient := &http.Client{Timeout: 10 * time.Second}
	for _, url := range instance.Spec.ImagestreamURLs {
		logMsg := processImageStreamURL(url, httpclient, imageclient, instance)
		if len(logMsg) > 0 {
			reqLogger.Info(logMsg)
		}
	}
	for _, url := range instance.Spec.TemplateURLs {
		logMsg := processTemplateURL(url, httpclient, templateclient, instance)
		if len(logMsg) > 0 {
			reqLogger.Info(logMsg)
		}
	}

	return reconcile.Result{Requeue: true, RequeueAfter: interval}, nil
}

func processTemplateURL(url string,
	httpclient *http.Client,
	templateclient *templateset.Clientset,
	instance *olmv1alpha1.Samples) string {

	r, err := httpclient.Get(url)
	if err != nil {
		return fmt.Sprintf("Error getting template content from %s", url)
	}
	defer r.Body.Close()
	t := &templatev1.Template{}
	err = json.NewDecoder(r.Body).Decode(t)
	if err != nil {
		return fmt.Sprintf("Error decoding template body %s", err.Error())
	}
	t, err = templateclient.TemplateV1().Templates(ns).Create(t)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Sprintf("Error creating template %s", err.Error())
		}
		if instance.Spec.UpdateTemplates {
			t, err = templateclient.TemplateV1().Templates(ns).Update(t)
			if err != nil {
				return fmt.Sprintf("Error updating template %s", err.Error())
			}
		}
	}
	return ""
}

func processImageStreamURL(url string,
	httpclient *http.Client,
	imageclient *imageset.Clientset,
	instance *olmv1alpha1.Samples) string {

	retMsg := ""
	r, err := httpclient.Get(url)
	if err != nil {
		return fmt.Sprintf("Error getting imagesream content from %s", url)
	}
	defer r.Body.Close()
	is := &imageapiv1.ImageStream{}
	err = json.NewDecoder(r.Body).Decode(is)
	if err != nil {
		return fmt.Sprintf("Error decoding imagestream body %s", err.Error())
	}
	name := is.Name
	is, err = imageclient.ImageV1().ImageStreams(ns).Create(is)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Sprintf("Error creating imagestream %s", err.Error())
		}
		if instance.Spec.UpdateImagestreams {
			is, err = imageclient.ImageV1().ImageStreams(ns).Update(is)
			if err != nil {
				return fmt.Sprintf("Error updating imagestream %s", err.Error())
			}
		}
	}
	if instance.Spec.RetryFailedImports {
		//TODO or can we use the return from create regardless of error
		is, err = imageclient.ImageV1().ImageStreams(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return fmt.Sprintf("Error getting imagestream %s: %s", name, err.Error())
		}
		for _, statusTag := range is.Status.Tags {
			// if an error occurred with the latest generation, let's give up as we are no longer "in progress"
			// in that case as well, but mark the import failure
			if statusTag.Conditions != nil && len(statusTag.Conditions) > 0 {
				var latestGeneration int64
				var mostRecentErrorGeneration int64
				for _, condition := range statusTag.Conditions {
					if condition.Generation > latestGeneration {
						latestGeneration = condition.Generation
					}
					if condition.Status == corev1.ConditionFalse {
						if condition.Generation > mostRecentErrorGeneration {
							mostRecentErrorGeneration = condition.Generation
						}
					}
				}
				if mostRecentErrorGeneration > 0 && mostRecentErrorGeneration >= latestGeneration {
					imgImport, err := importTag(is, statusTag.Tag)
					if err != nil {
						retMsg = retMsg + fmt.Sprintf("\nProblem constructing import object for imagestream %s and tag %s", is.Name, statusTag.Tag)
						continue
					}
					imgImport, err = imageclient.ImageV1().ImageStreamImports(ns).Create(imgImport)
					if err != nil {
						retMsg = retMsg + fmt.Sprintf("Problem submitting import object for imagestream %s and tag %s", is.Name, statusTag.Tag)
					}
				}
			}
		}
	}
	return retMsg
}

func importTag(stream *imageapiv1.ImageStream, tag string) (*imageapiv1.ImageStreamImport, error) {

	// follow any referential tags to the destination
	finalTag, existing, multiple, err := followTagReferenceV1(stream, tag)
	if err != nil {
		return nil, err
	}

	if existing == nil || existing.From == nil {
		return nil, nil
	}

	// disallow re-importing anything other than DockerImage
	if existing.From != nil && existing.From.Kind != "DockerImage" {
		return nil, fmt.Errorf("tag %q points to existing %s %q, it cannot be re-imported", tag, existing.From.Kind, existing.From.Name)
	}

	// set the target item to import
	if multiple {
		tag = finalTag
	}

	// and create accompanying ImageStreamImport
	return newImageStreamImportTags(stream, map[string]string{tag: existing.From.Name}), nil
}

func followTagReferenceV1(stream *imageapiv1.ImageStream, tag string) (finalTag string, ref *imageapiv1.TagReference, multiple bool, err error) {
	seen := sets.NewString()
	for {
		if seen.Has(tag) {
			// circular reference; should not exist with samples but we sanity check
			// to avoid infinite loop
			return tag, nil, multiple, fmt.Errorf("circular reference stream %s tag %s", stream.Name, tag)
		}
		seen.Insert(tag)

		var tagRef imageapiv1.TagReference
		for _, t := range stream.Spec.Tags {
			if t.Name == tag {
				tagRef = t
				break
			}
		}

		if tagRef.From == nil || tagRef.From.Kind != "ImageStreamTag" {
			// terminating tag
			return tag, &tagRef, multiple, nil
		}

		// The reference needs to be followed with two format patterns:
		// a) sameis:sometag and b) sometag
		if strings.Contains(tagRef.From.Name, ":") {
			tagref := splitImageStreamTag(tagRef.From.Name)
			// sameis:sometag - follow the reference as sometag
			tag = tagref
		} else {
			// sometag - follow the reference
			tag = tagRef.From.Name
		}
		multiple = true
	}
}

func newImageStreamImportTags(stream *imageapiv1.ImageStream, tags map[string]string) *imageapiv1.ImageStreamImport {
	isi := newImageStreamImport(stream)
	for tag, from := range tags {
		var insecure, scheduled bool
		oldTagFound := false
		var oldTag imageapiv1.TagReference
		for _, t := range stream.Spec.Tags {
			if t.Name == tag {
				oldTag = t
				oldTagFound = true
				break
			}
		}

		if oldTagFound {
			insecure = oldTag.ImportPolicy.Insecure
			scheduled = oldTag.ImportPolicy.Scheduled
		}
		isi.Spec.Images = append(isi.Spec.Images, imageapiv1.ImageImportSpec{
			From: corev1.ObjectReference{
				Kind: "DockerImage",
				Name: from,
			},
			To: &corev1.LocalObjectReference{Name: tag},
			ImportPolicy: imageapiv1.TagImportPolicy{
				Insecure:  insecure,
				Scheduled: scheduled,
			},
			ReferencePolicy: getReferencePolicy(),
		})
	}
	return isi
}

func newImageStreamImport(stream *imageapiv1.ImageStream) *imageapiv1.ImageStreamImport {
	isi := &imageapiv1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:            stream.Name,
			Namespace:       stream.Namespace,
			ResourceVersion: stream.ResourceVersion,
		},
		Spec: imageapiv1.ImageStreamImportSpec{Import: true},
	}
	return isi
}

func splitImageStreamTag(nameAndTag string) (tag string) {
	parts := strings.SplitN(nameAndTag, ":", 2)
	if len(parts) > 1 {
		tag = parts[1]
	}
	if len(tag) == 0 {
		tag = "latest"
	}
	return tag
}

func getReferencePolicy() imageapiv1.TagReferencePolicy {
	ref := imageapiv1.TagReferencePolicy{}
	ref.Type = imageapiv1.LocalTagReferencePolicy
	return ref
}
