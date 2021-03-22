package controllers

import (
	"context"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/output"

	options "github.com/breathbath/go_utils/v2/pkg/config"
	"github.com/sirupsen/logrus"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/models"

	"github.com/cloudradar-monitoring/rportcli/internal/pkg/api"
)

const (
	ClientID   = "clid"
	Local      = "local"
	Remote     = "remote"
	Scheme     = "scheme"
	ACL        = "acl"
	CheckPort  = "checkp"
	DefaultACL = "<<YOU CURRENT PUBLIC IP>>"
)

type TunnelRenderer interface {
	RenderTunnels(tunnels []*models.Tunnel) error
	RenderTunnel(t *models.Tunnel) error
	RenderDelete(s output.KvProvider) error
}

type IPProvider interface {
	GetIP(ctx context.Context) (string, error)
}

type TunnelController struct {
	Rport          *api.Rport
	TunnelRenderer TunnelRenderer
	IPProvider     IPProvider
}

func (cc *TunnelController) Tunnels(ctx context.Context) error {
	clResp, err := cc.Rport.Clients(ctx)
	if err != nil {
		return err
	}

	tunnels := make([]*models.Tunnel, 0)
	for _, cl := range clResp.Data {
		for _, t := range cl.Tunnels {
			t.Client = cl.ID
			tunnels = append(tunnels, t)
		}
	}

	return cc.TunnelRenderer.RenderTunnels(tunnels)
}

func (cc *TunnelController) Delete(ctx context.Context, clientID, tunnelID string) error {
	err := cc.Rport.DeleteTunnel(ctx, clientID, tunnelID)
	if err != nil {
		return err
	}

	err = cc.TunnelRenderer.RenderDelete(&models.OperationStatus{Status: "OK"})
	if err != nil {
		return err
	}

	return nil
}

func (cc *TunnelController) Create(ctx context.Context, params *options.ParameterBag) error {
	clientID := params.ReadString(ClientID, "")
	local := params.ReadString(Local, "")
	remote := params.ReadString(Remote, "")
	scheme := params.ReadString(Scheme, "")
	acl := params.ReadString(ACL, "")
	if acl == "" || acl == DefaultACL {
		ip, e := cc.IPProvider.GetIP(context.Background())
		if e != nil {
			logrus.Errorf("failed to fetch IP: %v", e)
		} else {
			acl = ip
		}
	}

	checkPort := params.ReadString(CheckPort, "")
	tun, err := cc.Rport.CreateTunnel(ctx, clientID, local, remote, scheme, acl, checkPort)
	if err != nil {
		return err
	}

	return cc.TunnelRenderer.RenderTunnel(tun.Data)
}
