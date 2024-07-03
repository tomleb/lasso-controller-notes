# Printed Columns

## DynamicClient with object included

When we request with the following `Accept` header:

```
application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io
```

and with the following url query

```
?includeObject=Object
```

we get the following for configmaps in `default` namespace:


```json
{
  "apiVersion": "meta.k8s.io/v1",
  "columnDefinitions": [
    {
      "description": "Name must be unique ...",
      "format": "name",
      "name": "Name",
      "priority": 0,
      "type": "string"
    },
    {
      "description": "Data contains the conf...",
      "format": "",
      "name": "Data",
      "priority": 0,
      "type": "string"
    },
    {
      "description": "CreationTimestamp is a t...",
      "format": "",
      "name": "Age",
      "priority": 0,
      "type": "string"
    }
  ],
  "items": [],
  "kind": "Table",
  "metadata": {
    "resourceVersion": "2794"
  },
  "rows": [
    {
      "cells": [
        "kube-root-ca.crt",
        1,
        "113m"
      ],
      "object": {
        "apiVersion": "v1",
        "data": {
          "ca.crt": "-----BEGIN CERTIFICATE-----\nMIIBdjCCAR2gAwIBAgIBADAKBggqhkjOPQQDAjAjMSEwHwYDVQQDDBhrM3Mtc2Vy\ndmVyLWNhQDE3MjAwMjE1MTIwHhcNMjQwNzAzMTU0NTEyWhcNMzQwNzAxMTU0NTEy\nWjAjMSEwHwYDVQQDDBhrM3Mtc2VydmVyLWNhQDE3MjAwMjE1MTIwWTATBgcqhkjO\nPQIBBggqhkjOPQMBBwNCAAR01mWHd+WZea1jtsUC6m28PADrg06fHrZfBivoGqUZ\nysVhBicq3OtCeyI0IfyXg9khHFJ1nKOb4wVoPSkFQQWjo0IwQDAOBgNVHQ8BAf8E\nBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUbvrIut2EBlix7Q2gyIuY\n5cJAKjgwCgYIKoZIzj0EAwIDRwAwRAIgZs03L0ZNnf/iDftj9KbEU0Fb98lqit5M\n358Kf5a6JpwCIDgk0+yFWa7i/l0yufr5WmiNHNs8c4IcNPJ8ZYOKHTxS\n-----END CERTIFICATE-----\n"
        },
        "kind": "ConfigMap",
        "metadata": {
          "annotations": {
            "kubernetes.io/description": "Contains a CA bundle that can be used to verify the kube-apiserver when using internal endpoints such as the internal service IP or kubernetes.default.svc. No other usage is guaranteed across distributions of Kubernetes clusters."
          },
          "creationTimestamp": "2024-07-03T15:45:31Z",
          "managedFields": [
            {
              "apiVersion": "v1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {
                "f:data": {
                  ".": {},
                  "f:ca.crt": {}
                },
                "f:metadata": {
                  "f:annotations": {
                    ".": {},
                    "f:kubernetes.io/description": {}
                  }
                }
              },
              "manager": "k3s",
              "operation": "Update",
              "time": "2024-07-03T15:45:31Z"
            }
          ],
          "name": "kube-root-ca.crt",
          "namespace": "default",
          "resourceVersion": "429",
          "uid": "30fb8552-33fe-42a6-954a-621103417d08"
        }
      }
    }
  ]
}
```

## Without object included

When we request with the following `Accept` header:

```
application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io
```

and with the following url query

```
?includeObject=None
```

we get the following for configmaps in `default` namespace:

```json
{
  "apiVersion": "meta.k8s.io/v1",
  "columnDefinitions": [
    {
      "description": "Name must be unique ...",
      "format": "name",
      "name": "Name",
      "priority": 0,
      "type": "string"
    },
    {
      "description": "Data contains the conf...",
      "format": "",
      "name": "Data",
      "priority": 0,
      "type": "string"
    },
    {
      "description": "CreationTimestamp is a t...",
      "format": "",
      "name": "Age",
      "priority": 0,
      "type": "string"
    }
  ],
  "items": [],
  "kind": "Table",
  "metadata": {
    "resourceVersion": "2794"
  },
  "rows": [
    {
      "cells": [
        "kube-root-ca.crt",
        1,
        "113m"
      ],
      "object": null
    }
  ]
}
```
