# Automatic Database Provisioning

## Motivation
As a developer, I should be able to request external resources for the application that I am developing with little to no human intervention. If I need a database, let me just ask for it and have some process spin that up for me. Taking it further, I shouldn't need to know what the credentials are to connect to that database. We shouldn't need to know. Not only is it annoying to deal with credentials, it's a security risk.

All of this should also be possible from outside of Kubernetes. Thus, this project supports both. We shouldn't expect devs to have to use Kubernetes to provision their resources. But it should be automatically provisioned onto Kubernetes when they go to actually deploy the application.

## Recipe for the Special Sauce

### 1. StatefuleMeshService CRD
We define our own **Custom Resource Definition** named StatefulMeshService. A StatefulMeshService is a service that defines both an application and some persistant storage, usually a database.

Using a StatefulMeshService might look a little something like the following:

```yaml
apiVersion: k2.com/v1
kind: StatefulMeshService
metadata:
  name: sentence
spec:
  name: sentence
  image: sentence
  volumeclass: mysql
  volumeplan: 5-7-14
  ```

**name**: The name of our application. This also includes what namespace the application will be created in as well as the endpoint it will be mounted at

**image**: The Docker image for our application. This is so Kubernetes knows which image to host in the cluster and the state of the application does not change from environment to environment.

**volumeclass**: The class of the volume. Examples of other classes could include postgres, sqlserver, mariadb, etc.

**volumeplan**: The plan of the volume. This usually relates to size and performance of the volume. In this case, it's a version.

### 2. StatefulMeshService Controller
The StatefulMeshService controller listens for CRUD operations on StatefulMeshService CRDs. If a new one is created, this means a new application and database should be provisioned. The StatefulMeshService controller is responsible for taking the specification as defined in the StatefulMeshService CRD and creating the necessary resources (Ingress, Service, Deployment, etc) to reprent the application inside of the Kubernetes cluster.

Included in these resources are two resources provided by the [Kubernetes Service Catalog](https://kubernetes.io/docs/concepts/extend-kubernetes/service-catalog). They are the **ServiceInstance** and the **ServiceBinding**

## Service Catalog
The Service Catalog is an extension of the Kubernetes API that makes it easier to use external (and internal) resources within a Kubernetes cluster. Once Service Catalog is installed onto the cluster, the **ServiceInstance** and **ServiceBinding** objects become available for use.

### ServiceInstance
When a new ServiceInstance is created, it talks to the ServiceBroker to create a new instance of what you're trying to provision. In our case, a database. Consider again from the example above

```yaml
...
  volumeclass: mysql
  volumeplan: 5-7-14
```

With this specification in our CRD, the Service Catalog will talk to the Service Broker and create a new MySQL instance and configure it so that it lines up with the 5-7-14 plan, however that is defined.

So at this point, the instance exists, but you can't talk to it.

### ServiceBinding
The ServiceBinding is where the magic really happens. A ServiceBinding requires a ServiceInstance. When a ServiceBinding request is made, the Service Catalog will ask the Broker to generate a username, password, host and whatever else the Broker decides to return so that you can connect to it.

The ServiceCatalog will then take the response of that Bind, and put it into a Kubernetes Secret. Once the credentials are put into a Secret, that secret can be mounted onto a Pod so the application can use it an an environment variable.

This means that you don't know what the username or the password is to connect to the database. In this project, the application handles that for you.

## Open Service Broker API
It should be highlighted that while Service Catalog makes it easy to interact with external resources to be used within a Kubernetes cluster. It is just that, a Kubernetes construct. But under the hood, it's using the [Open Service Broker API](https://www.openservicebrokerapi.org/) to actually communicate with the Broker

## Broker
Broker has been mentioned a couple times, and just so you're not confused, a broker is just a service that knows how to speak the Open Service Broker API. It knows how to Provision and Bind resources so that people can connect to the service it's brokering. Examples may include an Azure Broker, a broker that knows how to provision Azure resources. Another may be an AWS broker.

In the context of this project, I am using [minibroker](https://github.com/osbkit/minibroker)

## Local Provisioning
To provision resources without Kubernetes, Service Catalog has created [svcat](https://github.com/kubernetes-incubator/service-catalog/tree/master/cmd/svcat).

Once installed you can then provision and bind resources with the available brokers.

To provision a new mysqldb:
```
svcat provision mysqldb --class mysql --plan 5-7-14 -p mysqlUser=admin -p mysqlPassword=admin
```

To bind to the instance (i.e. get credentials):
```
svcat bind mysqldb
```