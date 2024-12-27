package credential

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/prokraft/redbus/internal/config"
)

type Algo string

const (
	Sha256 Algo = "sha256"
	Sha512 Algo = "sha512"
)

func FromConf(conf config.KafkaCredentialsConfig) *Conf {
	if conf.Algo == "" {
		return nil
	}
	return &Conf{
		Algo:     Algo(conf.Algo),
		User:     conf.User,
		Password: conf.Password,
		Cert:     conf.Cert,
	}
}

func (a Algo) ToScamAlgo() *scram.Algorithm {
	switch a {
	case Sha256:
		return &scram.SHA256
	case Sha512:
		return &scram.SHA512
	default:
		return nil
	}
}

type Conf struct {
	Algo     Algo
	User     string
	Password string
	Cert     string
}

func (c Conf) GetSaslAndTls(ctx context.Context) (*sasl.Mechanism, *tls.Config, error) {
	algo := c.Algo.ToScamAlgo()
	if algo == nil {
		return nil, nil, fmt.Errorf("Unknown kafka auth algo: %v\n", c.Algo)
	}
	mechanism, err := scram.Mechanism(*algo, c.User, c.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("Can't create transport mechanism, algo: %v, user: %v, err: %w\n", c.Algo, c.User, err)
	}

	var tlsConfig *tls.Config
	if c.Cert != "" {
		certs := x509.NewCertPool()
		pemPath := c.Cert
		pemData, err := os.ReadFile(pemPath)
		if err != nil {
			return nil, nil, fmt.Errorf("Can't read pem file for kafka producer, path: %v, err: %w\n", pemPath, err)
		}
		certs.AppendCertsFromPEM(pemData)
		tlsConfig = &tls.Config{RootCAs: certs}
	}

	return &mechanism, tlsConfig, nil
}

func (c Conf) String() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf(
		"%s@%s:%s%s%s",
		c.Algo, c.User,
		string(c.Password[0]), strings.Repeat("*", len(c.Password)-2), string(c.Password[len(c.Password)-1])),
	)
	if c.Cert != "" {
		s.WriteString("@" + c.Cert)
	}
	return s.String()
}
