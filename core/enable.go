package zrokcore
import (
	"github.com/openziti/zrok/environment"
	"github.com/openziti/zrok/environment/env_core"
	"github.com/openziti/zrok/sdk/golang/sdk"
)
func Enable(rootDir, accountToken, description string) string {
	environment.SetRootDirName(rootDir)
	root, err := environment.LoadRoot()
	if err != nil {
		return jerr(err)
	}
	if root.IsEnabled() {
		return jok(root.Environment().ZitiIdentity)
	}
	if err := root.SetEnvironment(&env_core.Environment{
		AccountToken: accountToken,
		ApiEndpoint:  apiEndpoint,
	}); err != nil {
		return jerr(err)
	}
	env, err := sdk.EnableEnvironment(root, &sdk.EnableRequest{
	})
	if err != nil {
		return jerr(err)
	}
	if err := root.SetEnvironment(&env_core.Environment{
		AccountToken: accountToken,
		ZitiIdentity: env.ZitiIdentity,
		ApiEndpoint:  apiEndpoint,
	}); err != nil {
		return jerr(err)
	}
	if err := root.SaveZitiIdentityNamed(root.EnvironmentIdentityName(), env.ZitiConfig); err != nil {
		return jerr(err)
	}
	return jok(env.ZitiIdentity)
}
