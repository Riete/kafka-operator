package kafka

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	middlewarev1alpha1 "github.com/riete/kafka-operator/pkg/apis/middleware/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_kafka")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Kafka Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKafka{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("kafka-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Kafka
	err = c.Watch(&source.Kind{Type: &middlewarev1alpha1.Kafka{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Kafka
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &middlewarev1alpha1.Kafka{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKafka implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileKafka{}

// ReconcileKafka reconciles a Kafka object
type ReconcileKafka struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Kafka object and makes changes based on the state read
// and what is in the Kafka.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKafka) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Kafka")

	// Fetch the Kafka instance
	instance := &middlewarev1alpha1.Kafka{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.Spec.Servers%2 == 0 {
		return reconcile.Result{}, fmt.Errorf("kafka cluster should always have an odd number of servers")
	}

	// Define a new service object
	service := newStatefulSetService(instance)

	// Set RedisCluster instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, service, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if service already exists
	serviceFound := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, serviceFound)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
		err = r.client.Create(context.TODO(), service)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Define a new statefulset object
	statefulset := newStatefulSet(instance)

	// Set RedisCluster instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, statefulset, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if statefulset already exists
	statefulsetFound := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: statefulset.Name, Namespace: statefulset.Namespace}, statefulsetFound)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new statefulset", "StatefulSet.Namespace", statefulset.Namespace, "StatefulSet.Name", statefulset.Name)
		err = r.client.Create(context.TODO(), statefulset)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// update kafka
	command := fmt.Sprintf("rm -rf /var/lib/kafka/lost+found && bin/kafka-server-start.sh config/server.properties"+
		" --override broker.id=${HOSTNAME##*-} --override log.dirs=/var/lib/kafka "+
		"--override zookeeper.connect=%s --override num.partitions=%d --override log.retention.hours=%d",
		instance.Spec.Zookeeper, instance.Spec.NumPartitions, instance.Spec.LogRetentionHours)
	statefulsetFound.Spec.Replicas = &instance.Spec.Servers
	statefulsetFound.Spec.Template.Spec.Containers[0].Command = []string{"bash", "-c", command}
	err = r.client.Update(context.TODO(), statefulsetFound)
	if err != nil {
		reqLogger.Error(err, "Failed to update StatefulSet", "StatefulSet.Namespace", statefulsetFound.Namespace, "StatefulSet.Name", statefulsetFound.Name)
		return reconcile.Result{}, err
	}
	reqLogger.Info("Update StatefulSet", "StatefulSet.Namespace", statefulsetFound.Namespace, "StatefulSet.Name", statefulsetFound.Name)

	// Define a kafka_exporter deployment
	if instance.Spec.Metrics {
		exporter := newKafkaExporterDeployment(instance)
		if err := controllerutil.SetControllerReference(instance, exporter, r.scheme); err != nil {
			return reconcile.Result{}, err
		}
		exporterFound := &appsv1.Deployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: exporter.Name, Namespace: exporter.Namespace}, exporterFound)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new deployment", "Deployment.Namespace", exporter.Namespace, "Deployment.Name", exporter.Name)
			err = r.client.Create(context.TODO(), exporter)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Kafka already exists - don't requeue
	reqLogger.Info("Skip reconcile: Kafka already exists", "Kafka.Namespace", instance.Namespace, "Kafka.Name", instance.Name)
	return reconcile.Result{}, nil
}

func newKafkaExporterDeployment(kfk *middlewarev1alpha1.Kafka) *appsv1.Deployment {
	var replicas int32 = 1
	cpuRequest, _ := resource.ParseQuantity("10m")
	cpuLimit, _ := resource.ParseQuantity("50m")
	memoryRequest, _ := resource.ParseQuantity("20Mi")
	memoryLimit, _ := resource.ParseQuantity("100Mi")
	name := fmt.Sprintf("%s-exporter", kfk.Name)
	labels := map[string]string{"app": name}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kfk.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"prometheus.io/path":   "/metrics",
						"prometheus.io/port":   strconv.Itoa(int(middlewarev1alpha1.KafkaExporterPort)),
						"prometheus.io/scrape": "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: middlewarev1alpha1.KafkaExporterImage,
						Name:  name,
						Args: []string{
							fmt.Sprintf("--kafka.server=%s:9092", kfk.Name),
							fmt.Sprintf("--kafka.version=%s", kfk.Spec.KafkaVersion),
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: middlewarev1alpha1.KafkaExporterPort,
							Name:          "kafka-exporter",
							Protocol:      corev1.ProtocolTCP,
						}},
						LivenessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(middlewarev1alpha1.KafkaExporterPort)),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(middlewarev1alpha1.KafkaExporterPort)),
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    cpuRequest,
								corev1.ResourceMemory: memoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    cpuLimit,
								corev1.ResourceMemory: memoryLimit,
							},
						},
					}},
				},
			},
		},
	}
}

func newStatefulSetService(kfk *middlewarev1alpha1.Kafka) *corev1.Service {
	labels := map[string]string{"app": kfk.Name}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kfk.Name,
			Namespace: kfk.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Type:      corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Name:       "client",
				Protocol:   corev1.ProtocolTCP,
				Port:       9092,
				TargetPort: intstr.FromInt(9092),
			}},
		},
	}
}

func newStatefulSet(kfk *middlewarev1alpha1.Kafka) *appsv1.StatefulSet {
	image := fmt.Sprintf("%s:%s", middlewarev1alpha1.REPOSITORY, kfk.Spec.Tag)
	labels := map[string]string{"app": kfk.Name}
	storageSize, _ := resource.ParseQuantity(kfk.Spec.StorageSize)
	heap := kfk.Spec.Heap
	if kfk.Spec.Heap == "" {
		heap = "-XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap -XX:MaxRAMFraction=2"
	}
	command := fmt.Sprintf("rm -rf /var/lib/kafka/lost+found && bin/kafka-server-start.sh config/server.properties"+
		" --override broker.id=${HOSTNAME##*-} --override log.dirs=/var/lib/kafka "+
		"--override zookeeper.connect=%s --override num.partitions=%d --override log.retention.hours=%d",
		kfk.Spec.Zookeeper, kfk.Spec.NumPartitions, kfk.Spec.LogRetentionHours)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kfk.Name,
			Namespace: kfk.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &kfk.Spec.Servers,
			ServiceName: kfk.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Command: []string{
							"bash",
							"-c",
							command,
						},
						Image: image,
						Name:  kfk.Name,
						Ports: []corev1.ContainerPort{{
							Name:          "client",
							Protocol:      corev1.ProtocolTCP,
							ContainerPort: 9092,
						}},
						LivenessProbe: &corev1.Probe{
							FailureThreshold:    6,
							InitialDelaySeconds: 90,
							PeriodSeconds:       30,
							SuccessThreshold:    1,
							TimeoutSeconds:      5,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(9092),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold:    6,
							InitialDelaySeconds: 90,
							PeriodSeconds:       30,
							SuccessThreshold:    1,
							TimeoutSeconds:      5,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(9092),
								},
							},
						},
						Resources: kfk.Spec.Resources,
						Env: []corev1.EnvVar{
							{Name: "KAFKA_HEAP_OPTS", Value: heap},
						},
					}},
				},
			},
		},
	}

	if kfk.Spec.StorageClass != "" {
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{Name: "data", MountPath: "/var/lib/kafka"},
		}
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "data",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageSize,
					},
				},
				StorageClassName: &kfk.Spec.StorageClass,
			},
		}}
	}

	return sts
}
