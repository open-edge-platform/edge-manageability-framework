// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"context"
	"errors"

	"github.com/Nerzal/gocloak/v13"

	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

const realm = "master"

func AddUserToGroup(ctx context.Context, username, groupName string) error {
	keycloakClient, token, err := util.KeycloakLogin(ctx)
	if err != nil {
		return err
	}

	group, err := keycloakClient.GetGroupByPath(ctx, token.AccessToken, realm, groupName)
	if err != nil {
		return err
	}

	userID, err := getUserID(ctx, keycloakClient, token.AccessToken, username)
	if err != nil {
		return err
	}

	err = keycloakClient.AddUserToGroup(ctx, token.AccessToken, realm, userID, *group.ID)
	if err != nil {
		return err
	}

	return nil
}

func AddRealmRoleToUser(ctx context.Context, username, roleName string) error {
	keycloakClient, token, err := util.KeycloakLogin(ctx)
	if err != nil {
		return err
	}

	keycloakRole, err := keycloakClient.GetRealmRole(ctx, token.AccessToken, realm, roleName)
	if err != nil {
		return err
	}

	userID, err := getUserID(ctx, keycloakClient, token.AccessToken, username)
	if err != nil {
		return err
	}

	roles := []gocloak.Role{*keycloakRole}

	return keycloakClient.AddRealmRoleToUser(ctx, token.AccessToken, realm, userID, roles)
}

func AddClientRoleToUser(ctx context.Context, username, roleName, clientID string) error {
	keycloakClient, token, err := util.KeycloakLogin(ctx)
	if err != nil {
		return err
	}

	clients, err := keycloakClient.GetClients(ctx, token.AccessToken, realm, gocloak.GetClientsParams{ClientID: &clientID})
	if err != nil {
		return err
	}

	var idOfClient string
	for _, v := range clients {
		if *v.ClientID == clientID {
			idOfClient = *v.ID
			break
		}
	}

	keycloakRole, err := keycloakClient.GetClientRole(ctx, token.AccessToken, realm, idOfClient, roleName)
	if err != nil {
		return err
	}

	userID, err := getUserID(ctx, keycloakClient, token.AccessToken, username)
	if err != nil {
		return err
	}

	roles := []gocloak.Role{*keycloakRole}

	return keycloakClient.AddClientRolesToUser(ctx, token.AccessToken, realm, idOfClient, userID, roles)
}

func CreateUser(ctx context.Context, userName string) error {
	user := &gocloak.User{
		Username:      &userName,
		FirstName:     &userName, // First name and last name is needed and cannot be empty for receivers endpoint to be working properly
		LastName:      &userName,
		Email:         gocloak.StringP(userName + "@observability-user.com"),
		Enabled:       gocloak.BoolP(true),
		EmailVerified: gocloak.BoolP(true),
	}

	keycloakClient, token, err := util.KeycloakLogin(ctx)
	if err != nil {
		return err
	}
	userID, err := keycloakClient.CreateUser(ctx, token.AccessToken, realm, *user)
	if err != nil {
		return err
	}

	defaultOrchPass, err := util.GetDefaultOrchPassword()
	if err != nil {
		return err
	}

	return keycloakClient.SetPassword(ctx, token.AccessToken, userID, realm, defaultOrchPass, false)
}

func getUserID(ctx context.Context, keycloakClient *gocloak.GoCloak, token, username string) (string, error) {
	params := gocloak.GetUsersParams{
		Username: &username,
	}

	users, err := keycloakClient.GetUsers(ctx, token, realm, params)
	if err != nil {
		return "", err
	}

	for _, user := range users {
		if *user.Username == username {
			return *user.ID, nil
		}
	}
	return "", errors.New("userID not found")
}
