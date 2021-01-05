![Build](https://github.com/mdvorak/ilo4-metrics-exporter/workflows/Build/badge.svg)

# ilo4-metrics-exporter

Simple proxy, providing temperatures from HPE iLO 4 as Prometheus metrics. Tested on versions 2.70 and 2.75.

It requires iLO account with read permissions.

## Deployment

Helm chart is available in the repository https://mdvorak.github.io/ilo4-metrics-exporter

To use it, add it first into repository list and then install it

```shell
helm repo add ilo4-metrics-exporter https://mdvorak.github.io/ilo4-metrics-exporter
helm repo update
helm install ilo4-metrics-exporter/ilo4-metrics-exporter
```

### Credentials

Important: In order to work, deployment needs secret of same name to be present, with `login.json` key, containing
following object:

```json
{
  "method": "login",
  "user_login": "myusername",
  "password": "mypassword"
}
```

_Note: User needs to have (and should have) readonly access only._

Secret sample:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ilo4-metrics-exporter
  namespace: ilo4-metrics-exporter
stringData:
  login.json: |
    {"method": "login", "user_login": "myusername", "password": "mypassword"}
```

It is up to you, how will you create the secret. 

If you are using, for example, [SealedSecrets](https://github.com/bitnami-labs/sealed-secrets), object can be
inlined into chart values, under `extraObjects` key, like this:

```yaml
ilo:
  url: "https://0.0.0.0"
# ...
extraObjects:
  - apiVersion: bitnami.com/v1alpha1
    kind: SealedSecret
    metadata:
      name: ilo4-metrics-exporter
      namespace: ilo4-metrics-exporter
    spec:
      encryptedData:
        login.json: abcd12343567752dsadasda...
```

_Note: You should never leak unencrypted secrets via helm install, have them in Git repository etc._

### Certificate

If iLo is using self-signed certificate (default), it needs to be added to exporter config (otherwise SSL error will be
thrown):

```yaml
ilo:
  url: "https://0.0.0.0"
  certificate: |
    -----BEGIN CERTIFICATE-----
    MIIC...
    -----END CERTIFICATE-----
```

### Security

Exporter does not need any security privileges, therefore it is recommended to set these in the values during
deployment:

```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
securityContext:
  capabilities:
    drop: [ ALL ]
  readOnlyRootFilesystem: true
```

_Note: These are not default in order to allow deployment in all kinds of environments._
