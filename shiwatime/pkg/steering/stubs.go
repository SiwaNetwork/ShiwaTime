package steering

import (
	"github.com/shiwatime/shiwatime/pkg/config"
)

// AlphaSteerer is a stub for Alpha algorithm
type AlphaSteerer struct {
	*SigmaSteerer
}

// NewAlphaSteerer creates a new Alpha steerer (currently uses Sigma implementation)
func NewAlphaSteerer(cfg config.SteeringConfig) (*AlphaSteerer, error) {
	sigma, err := NewSigmaSteerer(cfg)
	if err != nil {
		return nil, err
	}
	return &AlphaSteerer{SigmaSteerer: sigma}, nil
}

// GetName returns the algorithm name
func (a *AlphaSteerer) GetName() string {
	return "alpha"
}

// BetaSteerer is a stub for Beta algorithm
type BetaSteerer struct {
	*SigmaSteerer
}

// NewBetaSteerer creates a new Beta steerer (currently uses Sigma implementation)
func NewBetaSteerer(cfg config.SteeringConfig) (*BetaSteerer, error) {
	sigma, err := NewSigmaSteerer(cfg)
	if err != nil {
		return nil, err
	}
	return &BetaSteerer{SigmaSteerer: sigma}, nil
}

// GetName returns the algorithm name
func (b *BetaSteerer) GetName() string {
	return "beta"
}

// GammaSteerer is a stub for Gamma algorithm
type GammaSteerer struct {
	*SigmaSteerer
}

// NewGammaSteerer creates a new Gamma steerer (currently uses Sigma implementation)
func NewGammaSteerer(cfg config.SteeringConfig) (*GammaSteerer, error) {
	sigma, err := NewSigmaSteerer(cfg)
	if err != nil {
		return nil, err
	}
	return &GammaSteerer{SigmaSteerer: sigma}, nil
}

// GetName returns the algorithm name
func (g *GammaSteerer) GetName() string {
	return "gamma"
}

// RhoSteerer is a stub for Rho algorithm
type RhoSteerer struct {
	*SigmaSteerer
}

// NewRhoSteerer creates a new Rho steerer (currently uses Sigma implementation)
func NewRhoSteerer(cfg config.SteeringConfig) (*RhoSteerer, error) {
	sigma, err := NewSigmaSteerer(cfg)
	if err != nil {
		return nil, err
	}
	return &RhoSteerer{SigmaSteerer: sigma}, nil
}

// GetName returns the algorithm name
func (r *RhoSteerer) GetName() string {
	return "rho"
}