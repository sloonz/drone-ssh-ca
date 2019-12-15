package main

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"time"

	"github.com/drone/drone-go/drone"
	"github.com/gbrlsnchs/jwt/v3"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type spec struct {
	Bind         string `envconfig:"CA_BIND"`
	Debug        bool   `envconfig:"CA_DEBUG"`
	EnvPublicKey string `envconfig:"CA_ENV_PUBLIC_KEY"`
	PrivateKey   string `envconfig:"CA_PRIVATE_KEY"`
}

type buildPayload struct {
	jwt.Payload
	drone.Build
}

type repoPayload struct {
	jwt.Payload
	drone.Repo
}

func main() {
	var alg jwt.Algorithm

	spec := new(spec)
	err := envconfig.Process("", spec)
	if err != nil {
		logrus.Fatal(err)
	}

	if spec.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if spec.Bind == "" {
		spec.Bind = ":80"
	}

	block, _ := pem.Decode([]byte(spec.EnvPublicKey))
	if block == nil || block.Type != "PUBLIC KEY" {
		logrus.Fatalln("invalid public key")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logrus.Fatal(err)
	}

	privKey, err := ssh.ParseRawPrivateKey([]byte(spec.PrivateKey))
	if err != nil {
		logrus.Fatal(err)
	}

	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		logrus.Fatal(err)
	}

	switch k := pubKey.(type) {
	case ed25519.PublicKey:
		alg = jwt.NewEd25519(jwt.Ed25519PublicKey(k))
	case *rsa.PublicKey:
		alg = jwt.NewRS256(jwt.RSAPublicKey(k))
	case *ecdsa.PublicKey:
		alg = jwt.NewES256(jwt.ECDSAPublicKey(k))
	default:
		logrus.Fatalln("unsupported public key type")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var build buildPayload
		var repo repoPayload
		var err error

		r.ParseForm()

		_, err = jwt.Verify([]byte(r.Form["build"][0]), alg, &build)
		if err != nil {
			logrus.Warning(err)
			w.WriteHeader(400)
			return
		}

		_, err = jwt.Verify([]byte(r.Form["repo"][0]), alg, &repo)
		if err != nil {
			logrus.Warning(err)
			w.WriteHeader(400)
			return
		}

		sshPubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.Form["pubkey"][0]))
		if err != nil {
			logrus.Warningf("pubkey: %s", r.Form["pubkey"][0])
			logrus.Warning(err)
			w.WriteHeader(400)
			return
		}

		serialNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			logrus.Warning(err)
			w.WriteHeader(500)
			return
		}

		now := time.Now()
		cert := ssh.Certificate{
			Serial:   serialNumber.Uint64(),
			Key:      sshPubKey,
			CertType: ssh.UserCert,
			KeyId:    fmt.Sprintf("drone-%d", build.ID),
			ValidPrincipals: []string{
				"drone",
				fmt.Sprintf("drone:%s", repo.Namespace),
				fmt.Sprintf("drone:%s", repo.Slug),
				fmt.Sprintf("drone:%s:%s", repo.Slug, build.Target),
			},
			ValidAfter:  uint64(now.Add(-30 * time.Second).In(time.UTC).Unix()),
			ValidBefore: uint64(now.Add(1 * time.Hour).In(time.UTC).Unix()),
			Permissions: ssh.Permissions{
				Extensions: map[string]string{
					"permit-pty":              "",
					"permit-agent-forwarding": "",
					"permit-port-forwarding":  "",
				},
			},
		}

		err = cert.SignCert(rand.Reader, signer)
		if err != nil {
			logrus.Warning(err)
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write(ssh.MarshalAuthorizedKey(&cert))
	})

	logrus.Infof("server listening on address %s", spec.Bind)
	logrus.Fatal(http.ListenAndServe(spec.Bind, nil))
}
