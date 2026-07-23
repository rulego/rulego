package app

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/bboltstore"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/internal/store/jsonlstore"
	"github.com/rulego/rulego/server/internal/store/nopstore"
	"github.com/rulego/rulego/server/store"
)

// RegisterDefaultStoresHook registers the BeforeInit hook and injects the default file storage service into the container before module initialization.
// If a user has already injected a custom Provider via WithStoreProvider, automatic registration is skipped.
func RegisterDefaultStoresHook(application *App) {
	application.AddHook(NewFuncHook("register-stores", BeforeInit, 0,
		func(_ context.Context, appCtx *ModuleContext) error {
			cfg := appCtx.Config
			if cfg == nil {
				defaultCfg := config.DefaultConfig()
				cfg = &defaultCfg
			}
			logger := appCtx.Logger

			// If users inject a custom Provider via WithStoreProvider, use it directly
			if _, ok := appCtx.Container.Get("store.provider"); !ok {
				provider := filestore.NewFileStoreProvider(*cfg, logger)
				if err := appCtx.Container.Register("store.provider", store.StoreProvider(provider)); err != nil {
					return fmt.Errorf("register store provider: %w", err)
				}
			}

			// Injection into RunLogStore implementation
			provider, err := GetAs[store.StoreProvider](appCtx.Container, "store.provider")
			if err == nil {
				if fp, ok := provider.(*filestore.FileStoreProvider); ok {
					var runLogStore store.RunLogStore
					if !cfg.SaveRunLog {
						runLogStore = nopstore.NopRunLogStore{}
					} else {
						switch cfg.RunLogStoreType {
						case "file":
							runLogStore, err = jsonlstore.NewRunLogStore(*cfg, logger)
						default: // "bbolt" or empty
							runLogStore, err = bboltstore.NewRunLogStore(*cfg, logger)
						}
						if err != nil {
							return fmt.Errorf("create run log store: %w", err)
						}
					}
					fp.SetRunLogStore(runLogStore)
				}

				// Obtain UserStore registration from Provider to container (compatible with user module)
				if us, err := provider.GetUserStore(); err == nil {
					if err := appCtx.Container.Register("store.user", store.UserStore(us)); err != nil {
						return fmt.Errorf("register user store: %w", err)
					}
				}
			}
			return nil
		},
	))
}

// StartPprof Starts the pprof HTTP port according to the configuration, returns *http.Server (returns nil when not enabled).
func StartPprof(cfg *config.Config, logger types.Logger) *http.Server {
	if cfg == nil || !cfg.Pprof.Enable {
		return nil
	}
	addr := cfg.Pprof.Addr
	if addr == "" {
		addr = "0.0.0.0:6060"
	}
	srv := &http.Server{Addr: addr}
	go func() {
		logger.Infof("pprof listening on %s", addr)
		_ = srv.ListenAndServe()
	}()
	return srv
}
