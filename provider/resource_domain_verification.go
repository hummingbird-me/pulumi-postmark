package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/mrz1836/postmark"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// DomainVerification triggers Postmark's DKIM and Return-Path verification for a
// Domain. Model it to run *after* the DNS records produced by a Domain resource
// have been published (wire `domainId` from the Domain, and depend on your DNS
// records). By default it makes a single, non-blocking verification attempt;
// set `pollTimeoutSeconds` to wait for DNS propagation up to a bounded timeout.
type DomainVerification struct{}

var (
	_ infer.CustomResource[DomainVerificationArgs, DomainVerificationState] = (*DomainVerification)(nil)
	_ infer.CustomUpdate[DomainVerificationArgs, DomainVerificationState]   = (*DomainVerification)(nil)
	_ infer.CustomRead[DomainVerificationArgs, DomainVerificationState]     = (*DomainVerification)(nil)
	_ infer.CustomDelete[DomainVerificationState]                           = (*DomainVerification)(nil)
)

type DomainVerificationArgs struct {
	DomainID           int     `pulumi:"domainId" provider:"replaceOnChanges"`
	VerifyDKIM         *bool   `pulumi:"verifyDkim,optional"`
	VerifyReturnPath   *bool   `pulumi:"verifyReturnPath,optional"`
	Trigger            *string `pulumi:"trigger,optional"`
	PollTimeoutSeconds *int    `pulumi:"pollTimeoutSeconds,optional"`
}

type DomainVerificationState struct {
	DomainVerificationArgs
	DKIMVerified             bool   `pulumi:"dkimVerified"`
	ReturnPathDomainVerified bool   `pulumi:"returnPathDomainVerified"`
	DKIMHost                 string `pulumi:"dkimHost"`
	DKIMTextValue            string `pulumi:"dkimTextValue"`
}

func (a *DomainVerificationArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.DomainID, "ID of the Domain to verify (wire from `domain.domainId`). Immutable.")
	an.Describe(&a.VerifyDKIM, "Whether to verify DKIM. Defaults to true.")
	an.Describe(&a.VerifyReturnPath, "Whether to verify the Return-Path CNAME. Defaults to true.")
	an.Describe(&a.Trigger, "Arbitrary value; change it to force re-verification on the next update.")
	an.Describe(&a.PollTimeoutSeconds, "Maximum seconds to poll for verification to succeed while DNS "+
		"propagates. Defaults to 0 (a single, non-blocking attempt).")
}

func (*DomainVerification) Create(ctx context.Context, req infer.CreateRequest[DomainVerificationArgs]) (infer.CreateResponse[DomainVerificationState], error) {
	id := fmt.Sprintf("%d", req.Inputs.DomainID)
	if req.DryRun {
		return infer.CreateResponse[DomainVerificationState]{
			ID:     id,
			Output: DomainVerificationState{DomainVerificationArgs: req.Inputs},
		}, nil
	}
	client, err := accountClient(ctx)
	if err != nil {
		return infer.CreateResponse[DomainVerificationState]{}, err
	}
	state, err := runVerification(ctx, client, req.Inputs)
	if err != nil {
		return infer.CreateResponse[DomainVerificationState]{}, err
	}
	return infer.CreateResponse[DomainVerificationState]{ID: id, Output: state}, nil
}

func (*DomainVerification) Update(ctx context.Context, req infer.UpdateRequest[DomainVerificationArgs, DomainVerificationState]) (infer.UpdateResponse[DomainVerificationState], error) {
	if req.DryRun {
		out := req.State
		out.DomainVerificationArgs = req.Inputs
		return infer.UpdateResponse[DomainVerificationState]{Output: out}, nil
	}
	client, err := accountClient(ctx)
	if err != nil {
		return infer.UpdateResponse[DomainVerificationState]{}, err
	}
	state, err := runVerification(ctx, client, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[DomainVerificationState]{}, err
	}
	return infer.UpdateResponse[DomainVerificationState]{Output: state}, nil
}

func (*DomainVerification) Read(ctx context.Context, req infer.ReadRequest[DomainVerificationArgs, DomainVerificationState]) (infer.ReadResponse[DomainVerificationArgs, DomainVerificationState], error) {
	client, err := accountClient(ctx)
	if err != nil {
		return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{}, err
	}
	d, err := client.GetDomain(ctx, int64(req.Inputs.DomainID))
	if err != nil {
		if isNotFound(err) {
			return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{}, nil
		}
		return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{}, fmt.Errorf("reading domain %d for verification: %w", req.Inputs.DomainID, err)
	}
	state := req.State
	state.DomainVerificationArgs = req.Inputs
	applyDomainVerificationStatus(&state, d)
	return infer.ReadResponse[DomainVerificationArgs, DomainVerificationState]{
		ID:     req.ID,
		Inputs: req.Inputs,
		State:  state,
	}, nil
}

// Delete is a no-op: verification is an action, not a destroyable object.
func (*DomainVerification) Delete(_ context.Context, _ infer.DeleteRequest[DomainVerificationState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

func runVerification(ctx context.Context, client *postmark.Client, args DomainVerificationArgs) (DomainVerificationState, error) {
	domainID := int64(args.DomainID)
	verifyDKIM := deref(args.VerifyDKIM, true)
	verifyRP := deref(args.VerifyReturnPath, true)

	deadline := time.Now().Add(time.Duration(deref(args.PollTimeoutSeconds, 0)) * time.Second)
	const interval = 5 * time.Second

	for {
		var latest postmark.DomainDetails
		var err error
		ran := false
		if verifyDKIM {
			latest, err = client.VerifyDKIMStatus(ctx, domainID)
			if err != nil {
				return DomainVerificationState{}, fmt.Errorf("verifying DKIM for domain %d: %w", domainID, err)
			}
			ran = true
		}
		if verifyRP {
			latest, err = client.VerifyReturnPath(ctx, domainID)
			if err != nil {
				return DomainVerificationState{}, fmt.Errorf("verifying Return-Path for domain %d: %w", domainID, err)
			}
			ran = true
		}
		if !ran {
			latest, err = client.GetDomain(ctx, domainID)
			if err != nil {
				return DomainVerificationState{}, fmt.Errorf("reading domain %d: %w", domainID, err)
			}
		}

		done := (!verifyDKIM || latest.DKIMVerified) && (!verifyRP || latest.ReturnPathDomainVerified)
		if done || !time.Now().Before(deadline) {
			state := DomainVerificationState{DomainVerificationArgs: args}
			applyDomainVerificationStatus(&state, latest)
			return state, nil
		}

		select {
		case <-ctx.Done():
			return DomainVerificationState{}, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func applyDomainVerificationStatus(state *DomainVerificationState, d postmark.DomainDetails) {
	state.DKIMVerified = d.DKIMVerified
	state.ReturnPathDomainVerified = d.ReturnPathDomainVerified
	state.DKIMHost = d.DKIMHost
	state.DKIMTextValue = d.DKIMTextValue
}
