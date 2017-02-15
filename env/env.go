/*
 * This file is part of the MiniCloud project.
 * Copyright (C) 2017 Anton Frolov <frolov.anton@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package env

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var EtcdEndpoints string
var EtcdDialTimeout int64

func toEnvVarName(name string) string {
	elements := strings.Split(name, "-")
	for idx, elem := range elements {
		elements[idx] = strings.ToUpper(elem)
	}
	return "MINICLOUD_" + strings.Join(elements, "_")
}

func envStringVar(p *string, name string, value string, usage string) {
	if envValue, envSet := os.LookupEnv(toEnvVarName(name)); envSet {
		value = envValue
	}
	flag.StringVar(p, name, value, usage)
}

func envInt64Var(p *int64, name string, value int64, usage string) {
	envVarName := toEnvVarName(name)
	if envValue, envSet := os.LookupEnv(envVarName); envSet {
		if intEnvValue, err := strconv.ParseInt(envValue, 10, 64); err != nil {
			fmt.Errorf("Failed to parse %s value: %s", envVarName, err)
		} else {
			value = intEnvValue
		}
	}
	flag.Int64Var(p, name, value, usage)
}

func init() {
	envStringVar(&EtcdEndpoints, "etcd-endpoints", "127.0.0.1:2379", "Comma separated list of etcd endpoints")
	envInt64Var(&EtcdDialTimeout, "etcd-dial-timeout", 500, "Etcd connection timeout")
	flag.Parse()
}
