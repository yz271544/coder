//go:build !slim

package cli

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"net/url"
	"os"
	"time"
	"time"

	"golang.org/x/xerrors"
	"tailscale.com/derp"
	"tailscale.com/types/key"

	agplcoderd "github.com/coder/coder/v2/coderd"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbtime"
	"github.com/coder/coder/v2/codersdk"
	"github.com/coder/coder/v2/cryptorand"
	"github.com/coder/coder/v2/enterprise/aibridged"
	"github.com/coder/coder/v2/enterprise/audit"
	"github.com/coder/coder/v2/enterprise/audit/backends"
	"github.com/coder/coder/v2/enterprise/coderd"
	"github.com/coder/coder/v2/enterprise/coderd/dormancy"
	"github.com/coder/coder/v2/enterprise/coderd/license"
	"github.com/coder/coder/v2/enterprise/coderd/usage"
	"github.com/coder/coder/v2/enterprise/dbcrypt"
	"github.com/coder/coder/v2/enterprise/trialer"
	"github.com/coder/coder/v2/tailnet"
	"github.com/coder/quartz"
	"github.com/coder/serpent"
	"github.com/google/uuid"

	agplcoderd "github.com/coder/coder/v2/coderd"
)

func (r *RootCmd) Server(_ func()) *serpent.Command {
	cmd := r.RootCmd.Server(func(ctx context.Context, options *agplcoderd.Options) (*agplcoderd.API, io.Closer, error) {
		if options.DeploymentValues.DERP.Server.RelayURL.String() != "" {
			_, err := url.Parse(options.DeploymentValues.DERP.Server.RelayURL.String())
			if err != nil {
				return nil, nil, xerrors.Errorf("derp-server-relay-address must be a valid HTTP URL: %w", err)
			}
		}

		// Always generate a mesh key, even if the built-in DERP server is
		// disabled. This mesh key is still used by workspace proxies running
		// HA.
		var meshKey string
		err := options.Database.InTx(func(tx database.Store) error {
			// This will block until the lock is acquired, and will be
			// automatically released when the transaction ends.
			err := tx.AcquireLock(ctx, database.LockIDEnterpriseDeploymentSetup)
			if err != nil {
				return xerrors.Errorf("acquire lock: %w", err)
			}

			meshKey, err = tx.GetDERPMeshKey(ctx)
			if err == nil {
				return nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return xerrors.Errorf("get DERP mesh key: %w", err)
			}
			meshKey, err = cryptorand.String(32)
			if err != nil {
				return xerrors.Errorf("generate DERP mesh key: %w", err)
			}
			err = tx.InsertDERPMeshKey(ctx, meshKey)
			if err != nil {
				return xerrors.Errorf("insert DERP mesh key: %w", err)
			}
			return nil
		}, nil)
		if err != nil {
			return nil, nil, err
		}
		if meshKey == "" {
			return nil, nil, xerrors.New("mesh key is empty")
		}

		if options.DeploymentValues.DERP.Server.Enable {
			options.DERPServer = derp.NewServer(key.NewNode(), tailnet.Logger(options.Logger.Named("derp")))
			options.DERPServer.SetMeshKey(meshKey)
		}

		options.Auditor = audit.NewAuditor(
			options.Database,
			audit.DefaultFilter,
			backends.NewPostgres(options.Database, true),
			backends.NewSlog(options.Logger),
		)

		// Check if offline mode is enabled via environment variable
		offlineMode := os.Getenv("CODER_OFFLINE_MODE") == "true"
		offlineLicenseFile := os.Getenv("CODER_OFFLINE_LICENSE_FILE")
		offlinePublicKeyFile := os.Getenv("CODER_OFFLINE_PUBLIC_KEY_FILE")

		if offlineMode {
			// 在离线模式下，使用一个空的 TrialGenerator，不进行网络调用
			options.TrialGenerator = func(ctx context.Context, body codersdk.LicensorTrialRequest) error {
				options.Logger.Warn(ctx, "Offline mode enabled, skipping trial license generation")
				return nil
			}
		} else if offlineLicenseFile != "" && offlinePublicKeyFile != "" {
			// 如果指定了离线license文件，则读取并插入license
			licenseData, err := os.ReadFile(offlineLicenseFile)
			if err != nil {
				options.Logger.Error(ctx, "Failed to read offline license file", "error", err)
			} else {
				publicKeyContent, err := os.ReadFile(offlinePublicKeyFile)
				if err != nil {
					options.Logger.Error(ctx, "读取coder-publickey.pem文件失败: %v", err)
					return nil, nil, xerrors.Errorf("读取coder-publickey.pem文件失败: %v", err)
				}
				// 2. 解析PEM格式的公钥
				block, _ := pem.Decode(publicKeyContent)
				if block == nil || block.Type != "PUBLIC KEY" {
					return nil, nil, xerrors.Errorf("无效的PEM公钥文件: %v", offlinePublicKeyFile)
				}

				publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
				if err != nil {
					return nil, nil, xerrors.Errorf("解析公钥失败: %v", offlinePublicKeyFile)
				}

				ed25519PublicKey, ok := publicKey.(ed25519.PublicKey)
				if !ok {
					return nil, nil, xerrors.Errorf("不是ed25519公钥: %v", offlinePublicKeyFile)
				}

				keys := license.OfflineKeys
				keys[license.GetOfflineKeyID()] = ed25519PublicKey
				coderd.SetKeys(ed25519PublicKey)

				// 解析license
				jwt := string(licenseData)
				_, err = license.ParseClaims(jwt, keys)
				if err != nil {
					options.Logger.Error(ctx, "Failed to parse offline license", "error", err)
				} else {
					// 插入license到数据库
					id, _ := uuid.NewRandom()
					err = options.Database.InTx(func(tx database.Store) error {
						licenseByJwt, err := tx.GetLicenseByJWT(ctx, jwt)
						// License already exists, skip insertion
						if errors.Is(err, sql.ErrNoRows) {
							_, err = tx.InsertLicense(ctx, database.InsertLicenseParams{
								UploadedAt: dbtime.Now(),
								JWT:        jwt,
								Exp:        dbtime.Now().Add(time.Hour * 24 * 365 * 10), // 10 years
								UUID:       id,
							})
							return err
						} else {
							if err != nil {
								return xerrors.Errorf("check existing license: %w", err)
							}
							options.Logger.Info(ctx, "Offline license already exists in database, skipping insertion, id: "+licenseByJwt.UUID.String())
						}
						return nil
					}, nil)
					if err != nil {
						options.Logger.Error(ctx, "Failed to insert offline license into database", "error", err)
					} else {
						options.Logger.Info(ctx, "Successfully loaded offline license from file, id: "+id.String())
					}
				}
			}

			// 在使用离线license的情况下，也使用空的 TrialGenerator
			options.TrialGenerator = func(ctx context.Context, body codersdk.LicensorTrialRequest) error {
				options.Logger.Warn(ctx, "Offline license file provided, skipping trial license generation")
				return nil
			}
		} else {
			// 默认在线模式
			options.TrialGenerator = trialer.New(options.Database, "https://v2-licensor.coder.com/trial", coderd.Keys)
		}

		o := &coderd.Options{
			Options:                   options,
			AuditLogging:              true,
			ConnectionLogging:         true,
			BrowserOnly:               options.DeploymentValues.BrowserOnly.Value(),
			SCIMAPIKey:                []byte(options.DeploymentValues.SCIMAPIKey.Value()),
			RBAC:                      true,
			DERPServerRelayAddress:    options.DeploymentValues.DERP.Server.RelayURL.String(),
			DERPServerRegionID:        int(options.DeploymentValues.DERP.Server.RegionID.Value()),
			ProxyHealthInterval:       options.DeploymentValues.ProxyHealthStatusInterval.Value(),
			DefaultQuietHoursSchedule: options.DeploymentValues.UserQuietHoursSchedule.DefaultSchedule.Value(),
			ProvisionerDaemonPSK:      options.DeploymentValues.Provisioner.DaemonPSK.Value(),

			CheckInactiveUsersCancelFunc: dormancy.CheckInactiveUsers(ctx, options.Logger, quartz.NewReal(), options.Database, options.Auditor),
		}

		if encKeys := options.DeploymentValues.ExternalTokenEncryptionKeys.Value(); len(encKeys) != 0 {
			keys := make([][]byte, 0, len(encKeys))
			for idx, ek := range encKeys {
				dk, err := base64.StdEncoding.DecodeString(ek)
				if err != nil {
					return nil, nil, xerrors.Errorf("decode external-token-encryption-key %d: %w", idx, err)
				}
				keys = append(keys, dk)
			}
			cs, err := dbcrypt.NewCiphers(keys...)
			if err != nil {
				return nil, nil, xerrors.Errorf("initialize encryption: %w", err)
			}
			o.ExternalTokenEncryption = cs
		}

		if o.LicenseKeys == nil {
			o.LicenseKeys = coderd.Keys
		}

		closers := &multiCloser{}

		// Create the enterprise API.
		api, err := coderd.New(ctx, o)
		if err != nil {
			return nil, nil, err
		}
		closers.Add(api)

		// Start the enterprise usage publisher routine. This won't do anything
		// unless the deployment is licensed and one of the licenses has usage
		// publishing enabled.
		publisher := usage.NewTallymanPublisher(ctx, options.Logger, options.Database, o.LicenseKeys,
			usage.PublisherWithHTTPClient(api.HTTPClient),
		)
		err = publisher.Start()
		if err != nil {
			_ = closers.Close()
			return nil, nil, xerrors.Errorf("start usage publisher: %w", err)
		}
		closers.Add(publisher)

		// usageCron are heartbeat events to the usage table. These events are eventually sent
		// to Tallyman.
		usageCron := usage.NewCron(quartz.NewReal(), options.Logger.Named("usage-cron"), options.Database, *options.UsageInserter.Load())
		// ai-seats heartbeats track the number of users that have used an AI feature.
		// These users consume a seat for the AI addon to our License.
		_ = usageCron.Register(usage.CronJob{
			Name:     "ai-seats",
			Interval: usage.AISeatsInterval,
			Jitter:   10 * time.Minute,
			Fn:       usage.AISeatsHeartbeat(options.Database),
		})
		usageCron.Start(ctx)
		closers.Add(usageCron)

		// In-memory aibridge daemon.
		// TODO(@deansheather): the lifecycle of the aibridged server is
		// probably better managed by the enterprise API type itself. Managing
		// it in the API type means we can avoid starting it up when the license
		// is not entitled to the feature.
		var aibridgeDaemon *aibridged.Server
		if options.DeploymentValues.AI.BridgeConfig.Enabled {
			aibridgeDaemon, err = newAIBridgeDaemon(api)
			if err != nil {
				return nil, nil, xerrors.Errorf("create aibridged: %w", err)
			}

			api.RegisterInMemoryAIBridgedHTTPHandler(aibridgeDaemon)

			// When running as an in-memory daemon, the HTTP handler is wired into the
			// coderd API and therefore is subject to its context. Calling Close() on
			// aibridged will NOT affect in-flight requests but those will be closed once
			// the API server is itself shutdown.
			closers.Add(aibridgeDaemon)
		}

		// In-memory AI Bridge Proxy daemon
		if options.DeploymentValues.AI.BridgeProxyConfig.Enabled.Value() {
			aiBridgeProxyServer, err := newAIBridgeProxyDaemon(api)
			if err != nil {
				_ = closers.Close()
				return nil, nil, xerrors.Errorf("create aibridgeproxyd: %w", err)
			}
			closers.Add(aiBridgeProxyServer)

			// Register the handler so coderd can serve the proxy endpoints.
			api.RegisterInMemoryAIBridgeProxydHTTPHandler(aiBridgeProxyServer.Handler())
		}

		return api.AGPL, closers, nil
	})

	cmd.AddSubcommands(
		r.dbcryptCmd(),
	)
	return cmd
}

type multiCloser struct {
	closers []io.Closer
}

var _ io.Closer = &multiCloser{}

func (m *multiCloser) Add(closer io.Closer) {
	m.closers = append(m.closers, closer)
}

func (m *multiCloser) Close() error {
	var errs []error
	for _, closer := range m.closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, xerrors.Errorf("close %T: %w", closer, err))
		}
	}
	return errors.Join(errs...)
}
