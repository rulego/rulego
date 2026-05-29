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

// RegisterDefaultStoresHook 注册 BeforeInit 钩子，在模块初始化前将默认的文件存储服务注入容器。
// 如果用户已通过 WithStoreProvider 注入自定义 Provider，则跳过自动注册。
func RegisterDefaultStoresHook(application *App) {
	application.AddHook(NewFuncHook("register-stores", BeforeInit, 0,
		func(_ context.Context, appCtx *ModuleContext) error {
			cfg := appCtx.Config
			if cfg == nil {
				defaultCfg := config.DefaultConfig()
				cfg = &defaultCfg
			}
			logger := appCtx.Logger

			// 如果用户通过 WithStoreProvider 注入了自定义 Provider，直接使用
			if _, ok := appCtx.Container.Get("store.provider"); !ok {
				provider := filestore.NewFileStoreProvider(*cfg, logger)
				if err := appCtx.Container.Register("store.provider", store.StoreProvider(provider)); err != nil {
					return fmt.Errorf("register store provider: %w", err)
				}
			}

			// 注入 RunLogStore 实现
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
						default: // "bbolt" 或空
							runLogStore, err = bboltstore.NewRunLogStore(*cfg, logger)
						}
						if err != nil {
							return fmt.Errorf("create run log store: %w", err)
						}
					}
					fp.SetRunLogStore(runLogStore)
				}

				// 从 Provider 获取 UserStore 注册到容器（兼容 user 模块）
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

// StartPprof 按配置启动 pprof HTTP 端口，返回 *http.Server（未启用时返回 nil）。
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
