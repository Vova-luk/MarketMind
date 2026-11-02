package config

import configutil "marketmind/utils/config"

func Load(path string) (*configutil.Reader, error) {
	return configutil.Load(path, "MARKETMIND_")
}
