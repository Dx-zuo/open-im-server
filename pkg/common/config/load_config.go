package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/openimsdk/tools/errs"
	"github.com/openimsdk/tools/utils/runtimeenv"
	"github.com/spf13/viper"
)

func Load(configDirectory string, configFileName string, envPrefix string, config any) error {
	if runtimeenv.RuntimeEnvironment() == KUBERNETES {
		mountPath := os.Getenv(MountConfigFilePath)
		if mountPath == "" {
			return errs.ErrArgs.WrapMsg(MountConfigFilePath + " env is empty")
		}

		return loadConfig(filepath.Join(mountPath, configFileName), envPrefix, config)
	}

	return loadConfig(filepath.Join(configDirectory, configFileName), envPrefix, config)
}

func loadConfig(path string, envPrefix string, config any) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return errs.WrapMsg(err, "failed to read config file", "path", path, "envPrefix", envPrefix)
	}

	// 显式绑定所有嵌套键的环境变量，解决 viper AutomaticEnv 对嵌套配置不生效的问题
	// 例如：object.aws.bucketURL -> IMENV_OPENIM_RPC_THIRD_OBJECT_AWS_BUCKETURL
	for _, key := range v.AllKeys() {
		envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		fullEnvKey := envPrefix + "_" + envKey
		_ = v.BindEnv(key, fullEnvKey)
	}

	// 从环境变量中发现 YAML 文件中不存在的配置项
	// 这允许通过环境变量添加新的配置项，而不需要修改 YAML 文件
	envPrefixUpper := strings.ToUpper(envPrefix) + "_"
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envName := parts[0]
		if !strings.HasPrefix(envName, envPrefixUpper) {
			continue
		}
		// 将环境变量名转换为配置键：IMENV_OPENIM_RPC_THIRD_OBJECT_AWS_ENDPOINT -> object.aws.endpoint
		keySuffix := strings.TrimPrefix(envName, envPrefixUpper)
		configKey := strings.ToLower(strings.ReplaceAll(keySuffix, "_", "."))
		// 绑定环境变量到配置键
		_ = v.BindEnv(configKey, envName)
	}

	if err := v.Unmarshal(config, func(config *mapstructure.DecoderConfig) {
		config.TagName = StructTagName
	}); err != nil {
		return errs.WrapMsg(err, "failed to unmarshal config", "path", path, "envPrefix", envPrefix)
	}
	return nil
}
