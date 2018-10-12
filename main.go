package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

var wakeupChannel, sleepChannel chan string

const (
	// FormatHeader name of the header used to extract the format
	FormatHeader = "X-Format"

	// CodeHeader name of the header used as source of the HTTP statu code to return
	CodeHeader = "X-Code"

	// ContentType name of the header that defines the format of the reply
	ContentType = "Content-Type"

	// OriginalURI name of the header with the original URL from NGINX
	OriginalURI = "X-Original-URI"

	// Namespace name of the header that contains information about the Ingress namespace
	Namespace = "X-Namespace"

	// IngressName name of the header that contains the matched Ingress
	IngressName = "X-Ingress-Name"

	// ServiceName name of the header that contains the matched Service in the Ingress
	ServiceName = "X-Service-Name"

	// ServicePort name of the header that contains the matched Service port in the Ingress
	ServicePort = "X-Service-Port"

	// ErrFilesPathVar is the name of the environment variable indicating
	// the location on disk of files served by the handler.
	ErrFilesPathVar = "ERROR_FILES_PATH"

	// ServiceName name of the header that contains the host directive in the Ingress
	HostName = "X-Hostname"

	// Schema name of the header that contains the request schema
	Schema = "X-Schema"
)

func main() {

	backendHost := flag.String("host", "redis-master:6379", "backend host url")
	backendPassword := flag.String("password", "npCYPR7uAt", "backend password")

	await := make(map[string]chan bool)

	flag.Parse()

	logrus.Info("Starting default backend")
	client := redis.NewClient(&redis.Options{
		Addr:     *backendHost,
		Password: *backendPassword,
		DB:       0, // use default DB
	})

	http.HandleFunc("/", errorHandler(client, await))

	// http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logrus.Info("Starting app with time sleep")
	http.ListenAndServe(fmt.Sprintf(":8080"), nil)
	close(wakeupChannel)
}

func waitForIt(client *redis.Client, namespace, ingress string, dontRedirect chan<- bool) bool {
	logrus.Infof("Waiting for awake state of app %s/%s", namespace, ingress)
	timeout := time.After(15 * time.Minute)
	tick := time.Tick(time.Second * 2)
	for {
		select {
		case <-tick:
			val, err := client.Get(fmt.Sprintf("sleeping:%s:%s", namespace, ingress)).Result()
			if err != nil {
				logrus.WithError(err).Errorf("Failed to get app status: %v", err)
			}
			if val == "awake" {
				dontRedirect <- false
				close(dontRedirect)
				return true
			}
		case <-timeout:
			logrus.Infof("Timeout while waiting for app %s/%s", namespace, ingress)
			dontRedirect <- true
			return false
		}
	}
}

func errorHandler(client *redis.Client, await map[string]chan bool) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// ext := "html"

		ingressName := r.Header.Get(IngressName)
		namespace := r.Header.Get(Namespace)
		schema := r.Header.Get(Schema)
		hostname := r.Header.Get(HostName)
		uri := r.Header.Get(OriginalURI)
		origianlURL := fmt.Sprintf("%s://%s%s", schema, hostname, uri)
		dontRedirect := true

		logrus.Infof("Original: %s", origianlURL)

		u1 := uuid.NewV4()
		logrus.Infof("Request ID %v", u1)
		if ingressName != "" {
			val, err := client.Get(fmt.Sprintf("sleeping:%s:%s", namespace, ingressName)).Result()
			if err != nil {
				if err != redis.Nil {
					panic(err)
				}
				fmt.Fprint(w, "App is sleeping but we didn't know =/")
				return
			}
			app := fmt.Sprintf("%s/%s", namespace, ingressName)
			switch val {
			case "sleeping":
				err := client.Publish("wakeup", fmt.Sprintf("%s/%s", namespace, ingressName)).Err()
				if err != nil {
					logrus.Errorf("Failed to publish wakeup message: %v", err)
				}
				if val, ok := await[app]; ok {
					logrus.Infof("Waiting for redirect %v", u1)
					dontRedirect = <-val
					logrus.Infof("Got redirectRequest %v", u1)
				} else {
					await[app] = make(chan bool)
					logrus.Infof("calling waitForIt %v", u1)
					dontRedirect = waitForIt(client, namespace, ingressName, await[app])
				}

				// fmt.Fprintf(w, "App %s is sleeping. Don't you worry, we will start it for you. It might take a few minutes...", r.Header.Get(IngressName))
			case "waking_up":
				if val, ok := await[app]; ok {
					logrus.Infof("Waiting for redirect %v", u1)
					dontRedirect = <-val
					logrus.Infof("Got redirectRequest %v", u1)
				} else {
					await[app] = make(chan bool)
					logrus.Infof("calling waitForIt %v", u1)
					dontRedirect = waitForIt(client, namespace, ingressName, await[app])
				}
			case "awake":
				logrus.Infof("Already awake, redirect request %v", u1)
				time.Sleep(500 * time.Millisecond)
				dontRedirect = false
			default:
				fmt.Fprintf(w, "Page not found - 404")
			}
			if dontRedirect {
				logrus.Infof("Deleting map key %v", u1)
				delete(await, app)
			} else {
				logrus.Infof("Redirecting request %v", u1)
				http.Redirect(w, r, origianlURL, http.StatusSeeOther)
			}
		}
		// Not collected by custom error
		fmt.Fprintf(w, "Page not found - 404")
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func int32Ptr(i int32) *int32 { return &i }
