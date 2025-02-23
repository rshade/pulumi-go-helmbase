// Copyright 2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helmbase

import (
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

const (
	FieldHelmStatusOutput = "status"
	FieldHelmOptionsInput = "helmOptions"
)

// Chart represents a strongly typed Helm Chart resource. For the most part,
// it merely participates in the Pulumi resource lifecycle (by virtue of extending
// pulumi.ComponentResource), but it also offers a few specific helper methods.
type Chart interface {
	pulumi.ComponentResource
	// Type returns the fully qualified Pulumi type token for this resource.
	Type() string
	// SetOutputs registers the resulting Helm Release child resource, after it
	// has been created and registered. This contains the Status, among other things.
	SetOutputs(out helmv3.ReleaseStatusOutput)
	// DefaultChartName returns the default name for this chart.
	DefaultChartName() string
	// DefaultRepo returns the default Helm repo URL for this chart.
	DefaultRepoURL() string
}

// ReleaseType added because it was deprecated upstream.
type ReleaseType struct {
	// If set, installation process purges chart on fail. `skipAwait` will be disabled automatically if atomic is used.
	Atomic *bool `pulumi:"atomic"`
	// Chart name to be installed. A path may be used.
	Chart string `pulumi:"chart"`
	// Allow deletion of new resources created in this upgrade when upgrade fails.
	CleanupOnFail *bool `pulumi:"cleanupOnFail"`
	// Create the namespace if it does not exist.
	CreateNamespace *bool `pulumi:"createNamespace"`
	// Run helm dependency update before installing the chart.
	DependencyUpdate *bool `pulumi:"dependencyUpdate"`
	// Add a custom description
	Description *string `pulumi:"description"`
	// Use chart development versions, too. Equivalent to version '>0.0.0-0'. If `version` is set, this is ignored.
	Devel *bool `pulumi:"devel"`
	// Prevent CRD hooks from, running, but run other hooks.  See helm install --no-crd-hook
	DisableCRDHooks *bool `pulumi:"disableCRDHooks"`
	// If set, the installation process will not validate rendered templates against the Kubernetes OpenAPI Schema
	DisableOpenapiValidation *bool `pulumi:"disableOpenapiValidation"`
	// Prevent hooks from running.
	DisableWebhooks *bool `pulumi:"disableWebhooks"`
	// Force resource update through delete/recreate if needed.
	ForceUpdate *bool `pulumi:"forceUpdate"`
	// Location of public keys used for verification. Used only if `verify` is true
	Keyring *string `pulumi:"keyring"`
	// Run helm lint when planning.
	Lint *bool `pulumi:"lint"`
	// The rendered manifests as JSON. Not yet supported.
	Manifest map[string]interface{} `pulumi:"manifest"`
	// Limit the maximum number of revisions saved per release. Use 0 for no limit.
	MaxHistory *int `pulumi:"maxHistory"`
	// Release name.
	Name *string `pulumi:"name"`
	// Namespace to install the release into.
	Namespace *string `pulumi:"namespace"`
	// Postrender command to run.
	Postrender *string `pulumi:"postrender"`
	// Perform pods restart during upgrade/rollback.
	RecreatePods *bool `pulumi:"recreatePods"`
	// If set, render subchart notes along with the parent.
	RenderSubchartNotes *bool `pulumi:"renderSubchartNotes"`
	// Re-use the given name, even if that name is already used. This is unsafe in production
	Replace *bool `pulumi:"replace"`
	// Specification defining the Helm chart repository to use.
	RepositoryOpts helmv3.RepositoryOpts `pulumi:"repositoryOpts"`
	// When upgrading, reset the values to the ones built into the chart.
	ResetValues *bool `pulumi:"resetValues"`
	// Names of resources created by the release grouped by "kind/version".
	ResourceNames map[string][]string `pulumi:"resourceNames"`
	// When upgrading, reuse the last release's values and merge in any overrides. If 'resetValues' is specified, this is ignored
	ReuseValues *bool `pulumi:"reuseValues"`
	// By default, the provider waits until all resources are in a ready state before marking the release as successful. Setting this to true will skip such await logic.
	SkipAwait *bool `pulumi:"skipAwait"`
	// If set, no CRDs will be installed. By default, CRDs are installed if not already present.
	SkipCrds *bool `pulumi:"skipCrds"`
	// Status of the deployed release.
	Status helmv3.ReleaseStatus `pulumi:"status"`
	// Time in seconds to wait for any individual kubernetes operation.
	Timeout *int `pulumi:"timeout"`
	// List of assets (raw yaml files). Content is read and merged with values. Not yet supported.
	ValueYamlFiles []pulumi.AssetOrArchive `pulumi:"valueYamlFiles"`
	// Custom values set for the release.
	Values map[string]interface{} `pulumi:"values"`
	// Verify the package before installing it.
	Verify *bool `pulumi:"verify"`
	// Specify the exact chart version to install. If this is not specified, the latest version is installed.
	Version *string `pulumi:"version"`
	// Will wait until all Jobs have been completed before marking the release as successful. This is ignored if `skipAwait` is enabled.
	WaitForJobs *bool `pulumi:"waitForJobs"`
}

// ChartArgs is a properly annotated structure (with `pulumi:""` and `json:""` tags)
// which carries the strongly typed argument payload for the given Chart resource.
type ChartArgs interface {
	R() **ReleaseType
}

// Construct is the RPC call that initiates the creation of a new Chart component. It
// creates, registers, and returns the resulting component object. This contains most of
// the boilerplate so that the calling component can be relatively simple.
func Construct(ctx *pulumi.Context, c Chart, typ, name string,
	args ChartArgs, inputs provider.ConstructInputs, opts pulumi.ResourceOption) (*provider.ConstructResult, error) {

	// Ensure we have the right token.
	if et := c.Type(); typ != et {
		return nil, errors.Errorf("unknown resource type %s; expected %s", typ, et)
	}

	// Blit the inputs onto the arguments struct.
	if err := inputs.CopyTo(args); err != nil {
		return nil, errors.Wrap(err, "setting args")
	}

	// Register our component resource.
	if err := ctx.RegisterComponentResource(typ, name, c, opts); err != nil {
		return nil, err
	}

	// Provide default values for the Helm Release, including the chart name, repository
	// to pull from, and blitting the strongly typed values into the weakly typed map.
	relArgs := args.R()
	if *relArgs == nil {
		*relArgs = &ReleaseType{}
	}
	InitDefaults(*relArgs, c.DefaultChartName(), c.DefaultRepoURL(), args)

	// Create the actual underlying Helm Chart resource.
	rel, err := helmv3.NewRelease(ctx, name+"-helm", To(*relArgs), pulumi.Parent(c))
	if err != nil {
		return nil, err
	}
	c.SetOutputs(rel.Status)

	// Finally, register the resulting Helm Release as a component output.
	if err := ctx.RegisterResourceOutputs(c, pulumi.Map{
		FieldHelmStatusOutput: rel,
	}); err != nil {
		return nil, err
	}

	return provider.NewConstructResult(c)
}

// InitDefaults copies the default chart, repo, and values onto the args struct.
func InitDefaults(args *ReleaseType, chart, repo string, values interface{}) {
	// Most strongly typed charts will have a default chart name as well as a default
	// repository location. If available, set those. The user might override these,
	// so only initialize them if they're empty.
	if args.Chart == "" {
		args.Chart = chart
	}
	if args.RepositoryOpts.Repo == nil {
		args.RepositoryOpts.Repo = &repo
	}

	// Blit the strongly typed values onto the weakly typed values, so that the Helm
	// Release is constructed properly. In the event a value is present in both, the
	// strongly typed values override the weakly typed map.
	if args.Values == nil {
		args.Values = make(map[string]interface{})
	}

	// Decode the structure into the target map so we can copy it over to the values
	// map, which is what the Helm Release expects. We use the `pulumi:"x"`
	// tags to drive the naming of the resulting properties.
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:  &args.Values,
		TagName: "pulumi",
	})
	if err != nil {
		panic(err)
	}
	if err = d.Decode(values); err != nil {
		panic(err)
	}

	// Delete the HelmOptions input value -- it's not helpful and would cause a cycle.
	delete(args.Values, FieldHelmOptionsInput)
}

func toBoolPtr(p *bool) pulumi.BoolPtrInput {
	if p == nil {
		return nil
	}
	return pulumi.BoolPtr(*p)
}

func toIntPtr(p *int) pulumi.IntPtrInput {
	if p == nil {
		return nil
	}
	return pulumi.IntPtr(*p)
}

func toStringPtr(p *string) pulumi.StringPtrInput {
	if p == nil {
		return nil
	}
	return pulumi.StringPtr(*p)
}

func toAssetOrArchiveArray(a []pulumi.AssetOrArchive) pulumi.AssetOrArchiveArray {
	var res pulumi.AssetOrArchiveArray
	// TODO: ?!?!?!
	// cannot use e (variable of type pulumi.AssetOrArchive) as pulumi.AssetOrArchiveInput value in argument to append
	/*
		for _, e := range a {
			res = append(res, e)
		}
	*/
	return res
}

// To turns the args struct into a Helm-ready ReleaseArgs struct.
func To(args *ReleaseType) *helmv3.ReleaseArgs {
	// Create the Helm Release args.
	// TODO: it would be nice to do this automatically, e.g. using reflection, etc.
	//     This is caused by the helm.ReleaseArgs type not actually having the struct
	//     tags we need to use it directly (not clear why this is the case!)
	//     https://github.com/pulumi/pulumi/issues/8112
	return &helmv3.ReleaseArgs{
		Atomic:                   toBoolPtr(args.Atomic),
		Chart:                    pulumi.String(args.Chart),
		CleanupOnFail:            toBoolPtr(args.CleanupOnFail),
		CreateNamespace:          toBoolPtr(args.CreateNamespace),
		DependencyUpdate:         toBoolPtr(args.DependencyUpdate),
		Description:              toStringPtr(args.Description),
		Devel:                    toBoolPtr(args.Devel),
		DisableCRDHooks:          toBoolPtr(args.DisableCRDHooks),
		DisableOpenapiValidation: toBoolPtr(args.DisableOpenapiValidation),
		DisableWebhooks:          toBoolPtr(args.DisableWebhooks),
		ForceUpdate:              toBoolPtr(args.ForceUpdate),
		Keyring:                  toStringPtr(args.Keyring),
		Lint:                     toBoolPtr(args.Lint),
		Manifest:                 pulumi.ToMap(args.Manifest),
		MaxHistory:               toIntPtr(args.MaxHistory),
		Name:                     toStringPtr(args.Name),
		Namespace:                toStringPtr(args.Namespace),
		Postrender:               toStringPtr(args.Postrender),
		RecreatePods:             toBoolPtr(args.RecreatePods),
		RenderSubchartNotes:      toBoolPtr(args.RenderSubchartNotes),
		Replace:                  toBoolPtr(args.Replace),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			CaFile:   toStringPtr(args.RepositoryOpts.CaFile),
			CertFile: toStringPtr(args.RepositoryOpts.CertFile),
			KeyFile:  toStringPtr(args.RepositoryOpts.KeyFile),
			Password: toStringPtr(args.RepositoryOpts.Password),
			Repo:     toStringPtr(args.RepositoryOpts.Repo),
			Username: toStringPtr(args.RepositoryOpts.Username),
		},
		ResetValues:    toBoolPtr(args.ResetValues),
		ResourceNames:  pulumi.ToStringArrayMap(args.ResourceNames),
		ReuseValues:    toBoolPtr(args.ReuseValues),
		SkipAwait:      toBoolPtr(args.SkipAwait),
		SkipCrds:       toBoolPtr(args.SkipCrds),
		Timeout:        toIntPtr(args.Timeout),
		ValueYamlFiles: toAssetOrArchiveArray(args.ValueYamlFiles),
		Values:         pulumi.ToMap(args.Values),
		Verify:         toBoolPtr(args.Verify),
		Version:        toStringPtr(args.Version),
		WaitForJobs:    toBoolPtr(args.WaitForJobs),
	}
}
