package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	scv1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1alpha3 "github.com/michaelkipper/istio-client-go/pkg/apis/networking/v1alpha3"
	istiov1alpha3 "istio.io/api/networking/v1alpha3"
)

type Controller struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ControllerSpec   `json:"spec"`
	Status            ControllerStatus `json:"status"`
}

type ControllerSpec struct {
	Name        string `json:"name"`
	Image       string `json:"image"`
	VolumeClass string `json:"volumeclass"`
	VolumePlan  string `json:"volumeplan"`
}

type ControllerStatus struct {
	Replicas  int `json:"replicas"`
	Succeeded int `json:"succeeded"`
}

type SyncRequest struct {
	Parent   Controller          `json:"parent"`
	Children SyncRequestChildren `json:"children"`
}

type SyncRequestChildren struct {
	Namespaces  map[string]*corev1.Namespace     `json:"Namespace.v1"`
	Deployments map[string]*appsv1.Deployment    `json:"Deployment.apps/v1"`
	Services    map[string]*corev1.Service       `json:"Service.v1"`
	Ingresses   map[string]*extensionsv1.Ingress `json:"Ingress.extensions/v1beta1"`
}

type SyncResponse struct {
	Status   ControllerStatus `json:"status"`
	Children []runtime.Object `json:"children"`
}

func sync(request *SyncRequest) (*SyncResponse, error) {
	response := &SyncResponse{}

	serviceLabels := map[string]string{
		"app": request.Parent.Spec.Name,
	}

	namespaceLabel := map[string]string{
		"istio-injection": "enabled",
	}

	// Define the namespace for the application and its required resources to live in
	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   request.Parent.Spec.Name, // request.Parent.Spec.Name is the value defined on the CRD that was created
			Labels: namespaceLabel,
		},
	}

	// Define a new ServiceInstance that will ultimately provision the database
	serviceInstance := &scv1beta1.ServiceInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "servicecatalog.k8s.io/v1beta1",
			Kind:       "ServiceInstance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Parent.Spec.Name,
			Namespace: request.Parent.Spec.Name,
		},
		Spec: scv1beta1.ServiceInstanceSpec{
			PlanReference: scv1beta1.PlanReference{
				ClusterServiceClassExternalName: request.Parent.Spec.VolumeClass, // MySQL, postgres, etc
				ClusterServicePlanExternalName:  request.Parent.Spec.VolumePlan,  // Small, Medium, Large, Fast, Slow, whatever
			},
		},
	}

	// Once the instance is provisioned, create the binding to generate credentials
	serviceBinding := &scv1beta1.ServiceBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "servicecatalog.k8s.io/v1beta1",
			Kind:       "ServiceBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Parent.Spec.Name,
			Namespace: request.Parent.Spec.Name,
		},
		Spec: scv1beta1.ServiceBindingSpec{
			InstanceRef: scv1beta1.LocalObjectReference{
				Name: request.Parent.Spec.Name, // The name of the ServiceInstance that was created above
			},
			SecretName: request.Parent.Spec.Name, // The name of the Kubernetes Secret that we want the credentials to live in
		},
	}

	// The Ingress (in the form of an Istio VirtualService) to access the application
	virtualservice := &v1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.istio.io/v1alpha3",
			Kind:       "VirtualService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Parent.Spec.Name,
			Namespace: request.Parent.Spec.Name,
			Labels:    serviceLabels,
		},
		Spec: v1alpha3.VirtualServiceSpec{
			VirtualService: istiov1alpha3.VirtualService{
				Hosts: []string{
					"*",
				},
				Gateways: []string{
					"default/http-gateway",
				},
				Http: []*istiov1alpha3.HTTPRoute{
					&istiov1alpha3.HTTPRoute{
						Match: []*istiov1alpha3.HTTPMatchRequest{
							&istiov1alpha3.HTTPMatchRequest{
								Uri: &istiov1alpha3.StringMatch{
									MatchType: &istiov1alpha3.StringMatch_Prefix{
										Prefix: "/" + request.Parent.Spec.Name,
									},
								},
							},
						},
						Route: []*istiov1alpha3.HTTPRouteDestination{
							&istiov1alpha3.HTTPRouteDestination{
								Destination: &istiov1alpha3.Destination{
									Host: request.Parent.Spec.Name,
									Port: &istiov1alpha3.PortSelector{
										Port: &istiov1alpha3.PortSelector_Number{
											Number: uint32(80),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// The service that sits in front of the deployment pods for the application
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Parent.Spec.Name,
			Namespace: request.Parent.Spec.Name,
			Labels:    serviceLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 80,
				},
			},
			Selector: serviceLabels,
		},
	}

	// The deployment defintiion of the application
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Parent.Spec.Name,
			Namespace: request.Parent.Spec.Name,
			Labels:    serviceLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: serviceLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: serviceLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  request.Parent.Spec.Name,
							Image: request.Parent.Spec.Image + ":v1",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},

							/* These are the environment variables that will be mounted onto the container
							*  so that the application can use them to connect to the database */
							Env: []corev1.EnvVar{
								{
									Name: "DB_USERNAME", // create an environment variable called DB_USERNAME
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: request.Parent.Spec.Name,
											},
											Key: "username", // The key on the secret. In this case, the broker returns username
										},
									},
								},
								{
									Name: "DB_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: request.Parent.Spec.Name,
											},
											Key: "password",
										},
									},
								},
								{
									Name: "DB_HOST",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: request.Parent.Spec.Name,
											},
											Key: "host",
										},
									},
								},
								{
									Name: "DB_PORT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: request.Parent.Spec.Name,
											},
											Key: "port",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	// Return all of the resources to the Controller so that it can reconcile
	// any differences between the last request and this one
	response.Children = append(response.Children, namespace)

	response.Children = append(response.Children, serviceInstance)
	response.Children = append(response.Children, serviceBinding)

	response.Children = append(response.Children, deployment)
	response.Children = append(response.Children, service)
	response.Children = append(response.Children, virtualservice)

	return response, nil
}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	request := &SyncRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response, err := sync(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err = json.Marshal(&response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func main() {
	http.HandleFunc("/sync", syncHandler)
	log.Fatal(http.ListenAndServe(":80", nil))
}
