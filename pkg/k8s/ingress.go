package k8s

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type IngressHandlers struct {
	AddFunc    func(obj *v1beta1.Ingress)
	UpdateFunc func(oldObj, newObj *v1beta1.Ingress)
	DeleteFunc func(obj *v1beta1.Ingress)
}

type IngressParams struct {
	InformerFactory   informers.SharedInformerFactory
	ClassName         string
	ClassNameRequired bool
}

func WatchIngresses(options IngressParams, funcs IngressHandlers) cache.SharedIndexInformer {
	// TODO Handle new API
	informer := options.InformerFactory.Networking().V1beta1().Ingresses().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ingress, ok := obj.(*v1beta1.Ingress)

			if ok && IsControllerIngress(options, ingress) {
				funcs.AddFunc(ingress)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldIng, ok1 := oldObj.(*v1beta1.Ingress)
			newIng, ok2 := newObj.(*v1beta1.Ingress)

			if ok1 && ok2 && IsControllerIngress(options, newIng) {
				funcs.UpdateFunc(oldIng, newIng)
			}
		},
		DeleteFunc: func(obj interface{}) {
			ingress, ok := obj.(*v1beta1.Ingress)

			if ok && IsControllerIngress(options, ingress) {
				funcs.DeleteFunc(ingress)
			}
		},
	})

	return informer
}

func ListIngresses(options IngressParams) ([]*v1beta1.Ingress, error) {
	// TODO Handle new API
	ingresses, err := options.InformerFactory.Networking().V1beta1().Ingresses().Lister().List(labels.NewSelector())

	if err != nil {
		return nil, err
	}

	ings := []*v1beta1.Ingress{}
	for _, i := range ingresses {
		if IsControllerIngress(options, i) {
			ings = append(ings, i)
		}
	}
	return ings, nil
}

// IsControllerIngress check if the ingress object can be controlled by us
// TODO Handle `ingressClassName`
func IsControllerIngress(options IngressParams, ingress *v1beta1.Ingress) bool {
	ingressClass := ingress.Annotations["kubernetes.io/ingress.class"]
	if !options.ClassNameRequired && ingressClass == "" {
		return true
	}

	return ingressClass == options.ClassName
}

func UpdateIngressStatus(kubeClient *kubernetes.Clientset, ing *v1beta1.Ingress, status []apiv1.LoadBalancerIngress) (*v1beta1.Ingress, error) {
	ingClient := kubeClient.NetworkingV1beta1().Ingresses(ing.Namespace)

	currIng, err := ingClient.Get(ing.Name, v1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unexpected error searching Ingress %v/%v", ing.Namespace, ing.Name))
	}

	logrus.Debugf("updating Ingress %v/%v status from %v to %v", currIng.Namespace, currIng.Name, currIng.Status.LoadBalancer.Ingress, status)
	currIng.Status.LoadBalancer.Ingress = status

	return ingClient.UpdateStatus(currIng)
}
