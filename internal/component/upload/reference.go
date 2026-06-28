package upload

import (
	"context"
	"fmt"
	"time"
)

type InstantCommand struct {
	Profile      string
	OwnerUserID  uint64
	SHA256       string
	Size         int64
	OriginalName string
	ContentType  string
}

func (c *Component) InstantUpload(ctx context.Context, cmd InstantCommand) (*InstantResult, error) {
	profileName, profile, err := c.config.RequireProfile(cmd.Profile)
	if err != nil {
		return nil, err
	}
	if cmd.SHA256 == "" || cmd.Size <= 0 {
		return &InstantResult{Hit: false, ShouldUpload: true}, nil
	}
	if err := validateFileAttributes(profile, cmd.OriginalName, cmd.ContentType, cmd.Size); err != nil {
		return nil, err
	}
	reusable, err := c.store.FindReusableObject(ctx, FindReusableObjectCommand{
		SHA256:      cmd.SHA256,
		Size:        cmd.Size,
		OwnerUserID: cmd.OwnerUserID,
		Profile:     profileName,
	})
	if err != nil {
		return nil, err
	}
	if reusable == nil {
		return &InstantResult{Hit: false, ShouldUpload: true}, nil
	}
	expiresAt := time.Now().Add(profile.TTL)
	created, err := c.store.CreateInstantReference(ctx, CreateInstantReferenceCommand{
		FileID:      reusable.FileID,
		OwnerUserID: cmd.OwnerUserID,
		Profile:     profileName,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return nil, err
	}
	publicFileID, err := c.publicID(publicFileIDPrefix, created.FileID)
	if err != nil {
		return nil, err
	}
	publicReferenceID, err := c.publicID(publicReferenceIDPrefix, created.ReferenceID)
	if err != nil {
		return nil, err
	}
	accessURL, err := c.mustAccessURL(profileName, created.ReferenceID)
	if err != nil {
		return nil, err
	}
	return &InstantResult{
		Hit:          true,
		ShouldUpload: false,
		FileID:       publicFileID,
		ReferenceID:  publicReferenceID,
		Profile:      profileName,
		URL:          accessURL,
		ExpiresAt:    created.ExpiresAt,
	}, nil
}

func (c *Component) CommitReference(ctx context.Context, binding ReferenceBinding) (*ReferenceDetail, error) {
	if binding.ReferenceID == 0 {
		return nil, fmt.Errorf("upload: reference id is required")
	}
	profileName, _, err := c.config.RequireProfile(binding.Profile)
	if err != nil {
		return nil, err
	}
	return c.store.CommitReference(ctx, CommitReferenceCommand{
		ReferenceID:          binding.ReferenceID,
		OwnerUserID:          binding.OwnerUserID,
		Profile:              profileName,
		Service:              binding.Service,
		ResourceType:         binding.ResourceType,
		ResourceID:           binding.ResourceID,
		FieldName:            binding.FieldName,
		DeferReleaseExisting: binding.DeferReleaseExisting,
	})
}

func (c *Component) ReleasePreviousReferences(ctx context.Context, binding ReferenceBinding) error {
	if binding.ReferenceID == 0 {
		return fmt.Errorf("upload: reference id is required")
	}
	return c.store.ReleasePreviousReferences(ctx, ReleasePreviousReferencesCommand{
		ReferenceID:  binding.ReferenceID,
		Service:      binding.Service,
		ResourceType: binding.ResourceType,
		ResourceID:   binding.ResourceID,
		FieldName:    binding.FieldName,
	})
}

func (c *Component) GetReference(ctx context.Context, referenceID uint64, ownerUserID uint64) (*ReferenceDetail, error) {
	if referenceID == 0 {
		return nil, fmt.Errorf("upload: reference id is required")
	}
	return c.store.GetReference(ctx, referenceID, ownerUserID)
}

func (c *Component) GetBoundReference(ctx context.Context, binding ReferenceBinding) (*ReferenceDetail, error) {
	if binding.Service == "" || binding.ResourceType == "" || binding.ResourceID == 0 || binding.FieldName == "" {
		return nil, fmt.Errorf("upload: binding is incomplete")
	}
	profileName, _, err := c.config.RequireProfile(binding.Profile)
	if err != nil {
		return nil, err
	}
	return c.store.GetBoundReference(ctx, GetBoundReferenceCommand{
		Service:      binding.Service,
		ResourceType: binding.ResourceType,
		ResourceID:   binding.ResourceID,
		FieldName:    binding.FieldName,
		Profile:      profileName,
	})
}

func (c *Component) GetDownloadReference(ctx context.Context, referenceID uint64) (*DownloadObject, error) {
	if referenceID == 0 {
		return nil, fmt.Errorf("upload: reference id is required")
	}
	return c.store.GetDownloadReference(ctx, referenceID)
}

func (c *Component) GetDownloadReferenceForOwner(ctx context.Context, referenceID uint64, ownerUserID uint64) (*DownloadObject, error) {
	if referenceID == 0 {
		return nil, fmt.Errorf("upload: reference id is required")
	}
	return c.store.GetDownloadReferenceForOwner(ctx, referenceID, ownerUserID)
}

func (c *Component) ReleaseReference(ctx context.Context, referenceID uint64, ownerUserID uint64) error {
	if referenceID == 0 {
		return fmt.Errorf("upload: reference id is required")
	}
	return c.store.ReleaseReference(ctx, ReleaseReferenceCommand{
		ReferenceID: referenceID,
		OwnerUserID: ownerUserID,
	})
}
