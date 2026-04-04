package aws_test

import (
	"context"
	"errors"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/cloud"
	"github.com/stolostron/submariner-addon/pkg/cloud/aws"
	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/submariner-io/admiral/pkg/syncer/test"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	apifake "github.com/submariner-io/cloud-prepare/pkg/api/fake"
	cpaws "github.com/submariner-io/cloud-prepare/pkg/aws"
	cpclient "github.com/submariner-io/cloud-prepare/pkg/aws/client"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"github.com/submariner-io/submariner/pkg/cni"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/clock"
)

const (
	vpcName                   = "vpc-12345"
	controlPlaneSecurityGroup = "control-plane-sg-123"
	workerSecurityGroup       = "worker-sg-456"
	subnet1                   = "subnet-1"
	subnet2                   = "subnet-2"
)

var _ = Describe("AWS", func() {
	Describe("NewProvider", testNewProvider)
	Describe("PrepareSubmarinerClusterEnv", testPrepareSubmarinerClusterEnv)
	Describe("CleanUpSubmarinerClusterEnv", testCleanUpSubmarinerClusterEnv)
})

func testNewProvider() {
	t := newTestDriver()

	It("should correctly configure the AWS client", func(ctx context.Context) {
		t.assertNewProviderSuccess(ctx)

		Expect(t.capturedLoadOptions.Credentials).NotTo(BeNil())
		creds, err := t.capturedLoadOptions.Credentials.Retrieve(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(creds.AccessKeyID).To(Equal(string(t.info.CredentialsSecret.Data[aws.AccessKeyID])))
		Expect(creds.SecretAccessKey).To(Equal(string(t.info.CredentialsSecret.Data[aws.SecretAccessKey])))
	})

	It("should correctly configure the cloud prepare instance", func(ctx context.Context) {
		t.assertNewProviderSuccess(ctx)

		Expect(t.capturedRegion).To(Equal(t.info.Region))
		Expect(t.capturedInfraID).To(Equal(t.info.InfraID))
		Expect(t.capturedCloudOptions).To(Equal(cpaws.CloudOptions{
			VPCName:                   vpcName,
			ControlPlaneSecurityGroup: controlPlaneSecurityGroup,
			WorkerSecurityGroup:       workerSecurityGroup,
			PublicSubnetList:          []string{subnet1, subnet2},
		}))
	})

	It("should correctly configure the gateway deployer instance", func(ctx context.Context) {
		t.assertNewProviderSuccess(ctx)
		Expect(t.capturedInstanceType).To(Equal(aws.DefaultInstanceType))

		t.info.GatewayConfig.AWS.InstanceType = "custom-instance-type"

		t.assertNewProviderSuccess(ctx)
		Expect(t.capturedInstanceType).To(Equal(t.info.GatewayConfig.AWS.InstanceType))
	})

	When("the region is empty", func() {
		BeforeEach(func() {
			t.info.Region = ""
		})

		It("should return an error", func(ctx context.Context) {
			_, err := aws.NewProvider(ctx, t.info)
			Expect(err).To(HaveOccurred())
		})
	})

	When("the infraID is empty", func() {
		BeforeEach(func() {
			t.info.InfraID = ""
		})

		It("should return an error", func(ctx context.Context) {
			_, err := aws.NewProvider(ctx, t.info)
			Expect(err).To(HaveOccurred())
		})
	})

	When("the gateways count is 0", func() {
		BeforeEach(func() {
			t.info.GatewayConfig.Gateways = 0
		})

		It("should return an error", func(ctx context.Context) {
			_, err := aws.NewProvider(ctx, t.info)
			Expect(err).To(HaveOccurred())
		})
	})

	When("the AWS access key ID is missing", func() {
		BeforeEach(func() {
			delete(t.info.CredentialsSecret.Data, aws.AccessKeyID)
		})

		It("should return an error", func(ctx context.Context) {
			_, err := aws.NewProvider(ctx, t.info)
			Expect(err).To(HaveOccurred())
		})
	})

	When("the AWS secret access key is missing", func() {
		BeforeEach(func() {
			delete(t.info.CredentialsSecret.Data, aws.SecretAccessKey)
		})

		It("should return an error", func(ctx context.Context) {
			_, err := aws.NewProvider(ctx, t.info)
			Expect(err).To(HaveOccurred())
		})
	})

	When("AWS client creation fails", func() {
		BeforeEach(func() {
			aws.NewClient = func(ctx context.Context, region string, opts ...func(*awsconfig.LoadOptions) error) (cpclient.Interface, error) {
				return nil, errors.New("mock error")
			}
		})

		It("should return an error", func(ctx context.Context) {
			_, err := aws.NewProvider(ctx, t.info)
			Expect(err).To(HaveOccurred())
		})
	})

	When("gateway deployer creation fails", func() {
		BeforeEach(func() {
			aws.NewOcpGatewayDeployer = func(_ api.Cloud, _ ocp.MachineSetDeployer, _ string) (api.GatewayDeployer, error) {
				return nil, errors.New("mock error")
			}
		})

		It("should return an error", func(ctx context.Context) {
			_, err := aws.NewProvider(ctx, t.info)
			Expect(err).To(HaveOccurred())
		})
	})
}

//nolint:gosec // Ignore overflow conversion for port numbers
func testPrepareSubmarinerClusterEnv() {
	t := newTestDriver()

	It("should deploy gateways and open ports", func(ctx context.Context) {
		Expect(t.assertNewProviderSuccess(ctx).PrepareSubmarinerClusterEnv(ctx)).To(Succeed())

		Expect(t.gatewayDeployer.CapturedGatewayDeployInput).NotTo(BeNil())
		Expect(t.gatewayDeployer.CapturedGatewayDeployInput.Gateways).To(Equal(t.info.GatewayConfig.Gateways))
		Expect(t.gatewayDeployer.CapturedGatewayDeployInput.PublicPorts).To(ContainElement(
			api.PortSpec{Port: uint16(t.info.IPSecNATTPort), Protocol: "udp"},
		))
		Expect(t.gatewayDeployer.CapturedGatewayDeployInput.PublicPorts).To(ContainElement(
			api.PortSpec{Port: uint16(t.info.NATTDiscoveryPort), Protocol: "udp"},
		))

		Expect(t.cloud.CapturedPorts).To(ContainElement(
			api.PortSpec{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
		))
	})

	When("the CNI is OVNKubernetes", func() {
		BeforeEach(func() {
			t.info.NetworkType = cni.OVNKubernetes
		})

		It("should not open any ports", func(ctx context.Context) {
			Expect(t.assertNewProviderSuccess(ctx).PrepareSubmarinerClusterEnv(ctx)).To(Succeed())
			Expect(t.cloud.CapturedPorts).To(BeEmpty())
		})
	})

	When("gateway deployment fails", func() {
		BeforeEach(func() {
			t.gatewayDeployer.ReturnError = errors.New("mock error")
		})

		It("should return an error", func(ctx context.Context) {
			Expect(t.assertNewProviderSuccess(ctx).PrepareSubmarinerClusterEnv(ctx)).NotTo(Succeed())
		})
	})

	When("opening ports fails", func() {
		BeforeEach(func() {
			t.cloud.ReturnError = errors.New("mock error")
		})

		It("should return an error", func(ctx context.Context) {
			Expect(t.assertNewProviderSuccess(ctx).PrepareSubmarinerClusterEnv(ctx)).NotTo(Succeed())
		})
	})
}

func testCleanUpSubmarinerClusterEnv() {
	t := newTestDriver()

	It("should cleanup gateways and close ports", func(ctx context.Context) {
		Expect(t.assertNewProviderSuccess(ctx).CleanUpSubmarinerClusterEnv(ctx)).To(Succeed())
		Expect(t.gatewayDeployer.CleanupInvoked).To(BeTrue())
		Expect(t.cloud.CloseInvoked).To(BeTrue())
	})

	When("gateway cleanup fails", func() {
		BeforeEach(func() {
			t.gatewayDeployer.ReturnError = errors.New("mock error")
		})

		It("should return an error", func(ctx context.Context) {
			Expect(t.assertNewProviderSuccess(ctx).CleanUpSubmarinerClusterEnv(ctx)).NotTo(Succeed())
		})
	})

	When("closing ports fails", func() {
		BeforeEach(func() {
			t.cloud.ReturnError = errors.New("mock error")
		})

		It("should return an error", func(ctx context.Context) {
			Expect(t.assertNewProviderSuccess(ctx).CleanUpSubmarinerClusterEnv(ctx)).NotTo(Succeed())
		})
	})
}

type testDriver struct {
	info                 *provider.Info
	gatewayDeployer      *apifake.GatewayDeployer
	cloud                *apifake.Cloud
	capturedLoadOptions  awsconfig.LoadOptions
	capturedCloudOptions cpaws.CloudOptions
	capturedRegion       string
	capturedInfraID      string
	capturedInstanceType string
}

func newTestDriver() *testDriver {
	t := &testDriver{}

	BeforeEach(func() {
		t.gatewayDeployer = &apifake.GatewayDeployer{}
		t.cloud = &apifake.Cloud{}
		t.capturedLoadOptions = awsconfig.LoadOptions{}
		t.capturedCloudOptions = cpaws.CloudOptions{}
		t.capturedRegion = ""
		t.capturedInfraID = ""
		t.capturedInstanceType = ""

		aws.NewClient = func(ctx context.Context, region string, opts ...func(*awsconfig.LoadOptions) error) (cpclient.Interface, error) {
			for _, opt := range opts {
				Expect(opt(&t.capturedLoadOptions)).To(Succeed())
			}

			return cpclient.New(ctx, region, opts...)
		}

		aws.NewCloud = func(_ cpclient.Interface, infraID, region string, opts ...cpaws.CloudOption) api.Cloud {
			t.capturedRegion = region
			t.capturedInfraID = infraID

			for _, opt := range opts {
				opt(&t.capturedCloudOptions)
			}

			return t.cloud
		}

		aws.NewOcpGatewayDeployer = func(_ api.Cloud, _ ocp.MachineSetDeployer, instanceType string) (api.GatewayDeployer, error) {
			t.capturedInstanceType = instanceType
			return t.gatewayDeployer, nil
		}

		t.info = &provider.Info{
			RestMapper:    test.GetRESTMapperFor(),
			DynamicClient: fakedynamic.NewSimpleDynamicClient(scheme.Scheme),
			EventRecorder: events.NewLoggingEventRecorder("test", clock.RealClock{}),
			CredentialsSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-credentials",
					Namespace: "test-cluster",
				},
				Data: map[string][]byte{
					aws.AccessKeyID:     []byte("test-access-key-id"),
					aws.SecretAccessKey: []byte("test-secret-access-key"),
				},
			},
			SubmarinerConfigSpec: configv1alpha1.SubmarinerConfigSpec{
				IPSecNATTPort:     4600,
				NATTDiscoveryPort: 4491,
				GatewayConfig: configv1alpha1.GatewayConfig{
					Gateways: 2,
				},
			},
			ManagedClusterInfo: configv1alpha1.ManagedClusterInfo{
				ClusterName: "test-cluster",
				Platform:    "AWS",
				Region:      "us-east-1",
				InfraID:     "test-infra-id",
			},
			SubmarinerConfigAnnotations: map[string]string{
				aws.VPCNameKey:                   vpcName,
				aws.SubnetListKey:                subnet1 + "," + subnet2,
				aws.ControlPlaneSecurityGroupKey: controlPlaneSecurityGroup,
				aws.WorkerSecurityGroupKey:       workerSecurityGroup,
			},
		}
	})

	return t
}

func (t *testDriver) assertNewProviderSuccess(ctx context.Context) cloud.Provider {
	p, err := aws.NewProvider(ctx, t.info)
	Expect(err).NotTo(HaveOccurred())
	Expect(p).NotTo(BeNil())

	return p
}
