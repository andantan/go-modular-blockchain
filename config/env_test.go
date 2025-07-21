package config

import (
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInitEnv(t *testing.T) {
	err := godotenv.Load("../.env")

	assert.Nil(t, err)
}

func TestEnv(t *testing.T) {
	TestInitEnv(t)

	testString := GetEnvVar("TEST_STRING_VALUE")
	testInt := GetIntEnvVar("TEST_INTEGER_VALUE")
	testFloat := GetFloatEnvVar("TEST_FLOAT_VALUE")

	assert.Equal(t, "hello_world", testString)
	assert.Equal(t, 2147483647, testInt)
	assert.Equal(t, 3.14, testFloat)
}
