TODO: Make ingress work on localhost

## Objective

Your objective is run this rails application in PRODUCTION mode!

## Containers, Docker e Kubernetes

You must create containers to each of these services:

- Application
- PostgreSQL
- ElasticSearch
- Redis
- RabbitMQ

The orchestration of these containers must performed by Kubernetes.

### Set up PostgreSQL config

Available configurations

```Ruby
PG_DATABASE=...
PG_HOST=...
PG_USER=...
PG_PASSWORD=...
```

### Set up ElasticSearch config

Available configurations

```Ruby
ELASTICSEARCH_URL=...
```

### Set up RabbitMQ config

The RabbitMQ consumer performs using Sneakers gem. You will have to start the sneakers to  create an instance to start listening to the messages posted in RabbitMQ.

Available configurations

```Ruby
RABBITMQ_URL=...
RABBITMQ_EXCHANGE=amq.direct
RABBITMQ_QUEUE=...
```

#### Listening the messages

Start sneakers using this command:

```Ruby
WORKERS=Workers::LegislatorWorker bundle exec rake sneakers:run
```

### Set up Sidekiq/Redis config

Sidekiq is a tool to manage the async job. The application will run a job after receiving a RabbitMQ's message. Sidekiq uses the Redis to persist his queue, you must configure the Redis and connect to the application.

Available configurations

```Ruby
REDIS_URL=...
```

#### Run sidekiq

```Ruby
bundle exec sidekiq
```

### Set up Rails

You must run this command when run the first time the application, this command set up the Postgres database.

```Ruby
./bin/setup
```

### Run Rails

You should configure your application to run in PRODUCTION mode.
tips: you can use ngix + puma/unicorn

### To test if your application is working post message on RabbitMQ

Post a message in the RabbitMQ queue with this structure to present the data

```Ruby
{
  "name": "Legislator",
  "chamber": "house"
}
```

Check the result in the root's page of you application

`http://localhost:3000`

-------------------------LOCAL-SETUP-------------------------
------------
RabbitMQ
------------
- kubectl apply -f DEVOPS\rabbitmq\rabbit-namespace.yaml
- kubectl apply -f DEVOPS\rabbitmq\rabbit-secret.yaml -n rabbits
- kubectl apply -f DEVOPS\rabbitmq\rabbit-rbac.yaml -n rabbits
- kubectl apply -f DEVOPS\rabbitmq\rabbit-configmap.yaml -n rabbits
- kubectl apply -f DEVOPS\rabbitmq\rabbit-statefulset.yaml -n rabbits


------------
Redis && Sentinel
------------
- kubectl apply -f DEVOPS\redis\redis-configmap.yaml
- kubectl apply -f DEVOPS\redis\redis-service.yaml
- kubectl apply -f DEVOPS\redis\redis-statefulset.yaml
- kubectl apply -f DEVOPS\redis\sentinel\sentinel-statefulset.yaml


------------
ElasticSearch
------------
- kubectl apply -f DEVOPS\elasticsearch\es-statefulset.yaml
- kubectl apply -f DEVOPS\elasticsearch\es-service.yaml


------------
PostgreSQL && PGPool
------------
- kubectl apply -f DEVOPS\postgresql\postgres-namespace.yaml
- kubectl apply -f DEVOPS\postgresql\postgres-configmap.yaml -n database
- kubectl apply -f DEVOPS\postgresql\postgres-secrets.yaml -n database
- kubectl apply -f DEVOPS\postgresql\postgres-statefulset.yaml -n database
- kubectl apply -f DEVOPS\postgresql\postgres-headless-svc.yaml -n database
- kubectl apply -f DEVOPS\postgresql\pgpool\pgpool-secret.yaml -n database
- kubectl apply -f DEVOPS\postgresql\pgpool\pgpool-deployment.yaml -n database
- kubectl apply -f DEVOPS\postgresql\pgpool\pgpool-svc.yaml -n database
- kubectl apply -f DEVOPS\postgresql\pgpool\pgpool-svc-nodeport.yaml -n database


------------
### Rails-App
------------
# Build docker image
- docker build -t localhost:5000/my-image .

# Use created image to deploy our application
- kubectl apply -f DEVOPS\railsApp\app-namespace.yaml
- kubectl apply -f DEVOPS\railsApp\app-configmap.yaml
- kubectl apply -f DEVOPS\railsApp\app-deployment.yaml


-------------------------TESTING-------------------------
# Get pod-name to then run db restore
- kubectl get pods -o=name --field-selector=status.phase=Running  | head -1

# Now using the output of the last command, change the pod name
- kubectl exec CHANGE_THIS_POD_NAME_TO_PREVIOUS_COMMAND_OUTPUT -- rails db:setup

# Port-forward the connection to the pod and check the connection
- kubectl port-forward CHANGE_THIS_POD_NAME_TO_PREVIOUS_COMMAND_OUTPUT 3000:3000

# Create a message in rabbitmq-queue
- kubectl -n rabbits port-forward rabbitmq-0 8000:15672

# Open link in browser to see if rabbitmq is working ok.
# Note: Use user:guest pass:guest to login
http://localhost:8000/#/queues/
# Select the only queue that exists there and finally
# Choose Publish message and send this example
{
  "name": "Legislator",
  "chamber": "house"
}

