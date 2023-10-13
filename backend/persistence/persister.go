package persistence

import (
	"context"
	"embed"
	"fmt"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/gobuffalo/pop/v6"
	"github.com/teamhanko/hanko/backend/config"
	"os"
)

//go:embed migrations/*
var migrations embed.FS

// Persister is the persistence interface connecting to the database and capable of doing migrations
type persister struct {
	DB *pop.Connection
}

type Persister interface {
	GetConnection() *pop.Connection
	Transaction(func(tx *pop.Connection) error) error
	GetIdentityPersister() IdentityPersister
	GetIdentityPersisterWithConnection(tx *pop.Connection) IdentityPersister
	GetUserPersister() UserPersister
	GetUserPersisterWithConnection(tx *pop.Connection) UserPersister
	GetPasscodePersister() PasscodePersister
	GetPasscodePersisterWithConnection(tx *pop.Connection) PasscodePersister
	GetPasswordCredentialPersister() PasswordCredentialPersister
	GetPasswordCredentialPersisterWithConnection(tx *pop.Connection) PasswordCredentialPersister
	GetWebauthnCredentialPersister() WebauthnCredentialPersister
	GetWebauthnCredentialPersisterWithConnection(tx *pop.Connection) WebauthnCredentialPersister
	GetWebauthnSessionDataPersister() WebauthnSessionDataPersister
	GetWebauthnSessionDataPersisterWithConnection(tx *pop.Connection) WebauthnSessionDataPersister
	GetJwkPersister() JwkPersister
	GetJwkPersisterWithConnection(tx *pop.Connection) JwkPersister
	GetAuditLogPersister() AuditLogPersister
	GetAuditLogPersisterWithConnection(tx *pop.Connection) AuditLogPersister
	GetEmailPersister() EmailPersister
	GetEmailPersisterWithConnection(tx *pop.Connection) EmailPersister
	GetPrimaryEmailPersister() PrimaryEmailPersister
	GetPrimaryEmailPersisterWithConnection(tx *pop.Connection) PrimaryEmailPersister
	GetTokenPersister() TokenPersister
	GetTokenPersisterWithConnection(tx *pop.Connection) TokenPersister
	GetSessionPersister() SessionPersister
	GetSessionPersisterWithConnection(tx *pop.Connection) SessionPersister
	Health() error
	HealthWithConnection(tx *pop.Connection) error
}

type Migrator interface {
	MigrateUp() error
	MigrateDown(int) error
}

type Storage interface {
	Migrator
	Persister
}

// New return a new Persister Object with given configuration
func New(config config.Database) (Storage, error) {
	connectionDetails := &pop.ConnectionDetails{
		Pool:     5,
		IdlePool: 0,
		Options:  make(map[string]string),
	}

	password := config.Password
	if password == "AWS.IAM" {
		cfg, err := awsConfig.LoadDefaultConfig(context.Background())
		if err != nil {
			panic("configuration error: " + err.Error())
		}

		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "eu-central-1"
		}

		endpoint := fmt.Sprintf("%s:%s", config.Host, config.Port)
		fmt.Println("endpoint: ", endpoint)

		password, err = auth.BuildAuthToken(
			context.Background(),
			endpoint,
			region,
			config.User,
			cfg.Credentials,
		)
		if err != nil {
			panic("authentication error: " + err.Error())
		}

		connectionDetails.Options["sslmode"] = "require"
	}

	if len(config.Url) > 0 {
		connectionDetails.URL = config.Url
	} else {
		connectionDetails.Dialect = config.Dialect
		connectionDetails.Database = config.Database
		connectionDetails.Host = config.Host
		connectionDetails.Port = config.Port
		connectionDetails.User = config.User
		connectionDetails.Password = password
	}

	DB, err := pop.NewConnection(connectionDetails)

	if err != nil {
		return nil, err
	}

	if err := DB.Open(); err != nil {
		return nil, err
	}

	return &persister{
		DB: DB,
	}, nil
}

// MigrateUp applies all pending up migrations to the Database
func (p *persister) MigrateUp() error {
	migrationBox, err := pop.NewMigrationBox(migrations, p.DB)
	if err != nil {
		return err
	}
	err = migrationBox.Up()
	if err != nil {
		return err
	}
	return nil
}

// MigrateDown migrates the Database down by the given number of steps
func (p *persister) MigrateDown(steps int) error {
	migrationBox, err := pop.NewMigrationBox(migrations, p.DB)
	if err != nil {
		return err
	}
	err = migrationBox.Down(steps)
	if err != nil {
		return err
	}
	return nil
}

func (p *persister) GetConnection() *pop.Connection {
	return p.DB
}

func (p *persister) GetIdentityPersister() IdentityPersister {
	return NewIdentityPersister(p.DB)
}

func (p *persister) GetIdentityPersisterWithConnection(tx *pop.Connection) IdentityPersister {
	return NewIdentityPersister(tx)
}

func (p *persister) GetUserPersister() UserPersister {
	return NewUserPersister(p.DB)
}

func (p *persister) GetUserPersisterWithConnection(tx *pop.Connection) UserPersister {
	return NewUserPersister(tx)
}

func (p *persister) GetPasscodePersister() PasscodePersister {
	return NewPasscodePersister(p.DB)
}

func (p *persister) GetPasscodePersisterWithConnection(tx *pop.Connection) PasscodePersister {
	return NewPasscodePersister(tx)
}

func (p *persister) GetPasswordCredentialPersister() PasswordCredentialPersister {
	return NewPasswordCredentialPersister(p.DB)
}

func (p *persister) GetPasswordCredentialPersisterWithConnection(tx *pop.Connection) PasswordCredentialPersister {
	return NewPasswordCredentialPersister(tx)
}

func (p *persister) GetWebauthnCredentialPersister() WebauthnCredentialPersister {
	return NewWebauthnCredentialPersister(p.DB)
}

func (p *persister) GetWebauthnCredentialPersisterWithConnection(tx *pop.Connection) WebauthnCredentialPersister {
	return NewWebauthnCredentialPersister(tx)
}

func (p *persister) GetWebauthnSessionDataPersister() WebauthnSessionDataPersister {
	return NewWebauthnSessionDataPersister(p.DB)
}

func (p *persister) GetWebauthnSessionDataPersisterWithConnection(tx *pop.Connection) WebauthnSessionDataPersister {
	return NewWebauthnSessionDataPersister(tx)
}

func (p *persister) GetJwkPersister() JwkPersister {
	return NewJwkPersister(p.DB)
}

func (p *persister) GetJwkPersisterWithConnection(tx *pop.Connection) JwkPersister {
	return NewJwkPersister(tx)
}

func (p *persister) GetAuditLogPersister() AuditLogPersister {
	return NewAuditLogPersister(p.DB)
}

func (p *persister) GetAuditLogPersisterWithConnection(tx *pop.Connection) AuditLogPersister {
	return NewAuditLogPersister(tx)
}

func (p *persister) GetEmailPersister() EmailPersister {
	return NewEmailPersister(p.DB)
}

func (p *persister) GetEmailPersisterWithConnection(tx *pop.Connection) EmailPersister {
	return NewEmailPersister(tx)
}

func (p *persister) GetPrimaryEmailPersister() PrimaryEmailPersister {
	return NewPrimaryEmailPersister(p.DB)
}

func (p *persister) GetPrimaryEmailPersisterWithConnection(tx *pop.Connection) PrimaryEmailPersister {
	return NewPrimaryEmailPersister(tx)
}

func (p *persister) Transaction(fn func(tx *pop.Connection) error) error {
	return p.DB.Transaction(fn)
}

func (p *persister) GetTokenPersister() TokenPersister {
	return NewTokenPersister(p.DB)
}

func (p *persister) GetTokenPersisterWithConnection(tx *pop.Connection) TokenPersister {
	return NewTokenPersister(tx)
}

func (p *persister) GetSessionPersister() SessionPersister {
	return NewSessionPersister(p.DB)
}

func (p *persister) GetSessionPersisterWithConnection(tx *pop.Connection) SessionPersister {
	return NewSessionPersister(tx)
}

func (p *persister) Health() error {
	return p.DB.RawQuery("SELECT 1").Exec()
}

func (p *persister) HealthWithConnection(tx *pop.Connection) error {
	return tx.RawQuery("SELECT 1").Exec()
}
