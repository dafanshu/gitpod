/**
 * Copyright (c) 2021 Gitpod GmbH. All rights reserved.
 * Licensed under the GNU Affero General Public License (AGPL).
 * See License-AGPL.txt in the project root for license information.
 */

import "reflect-metadata";
import * as React from 'react';
import * as Cookies from 'js-cookie';

import { createGitpodService } from './service-factory';
import { ApplicationFrame } from "./components/page-frame";
import { GitpodHostUrl } from "@gitpod/gitpod-protocol/lib/util/gitpod-host-url";

import { renderEntrypoint } from "./entrypoint";

const service = createGitpodService();
const accountHints = (() => {
    try {
        const hints = Cookies.getJSON('ProceedWithAccountCookie');
        if (accountHints || "currentUser" in hints || "otherUser" in hints) {
            return hints;
        }
    } catch (error) {
        console.log(error);
    }
})();

export class ProceedWithAccount extends React.Component<{}, {}> {

    render() {
        if (!accountHints) {
            return (
                <ApplicationFrame service={service}>
                    <h3>Oh no! We are missing a cookie here. 🍪</h3>
                    <h2>A detailed errors message is missing.</h2>
                </ApplicationFrame>
            );
        }
        const { currentUser, otherUser } = accountHints;
        const loginUrl = new GitpodHostUrl(window.location.href).withApi({
            pathname: '/login/',
            search: `host=${otherUser.authHost}`
        }).toString();
        const logoutUrl = new GitpodHostUrl(window.location.toString()).withApi({
            pathname: "/logout",
            search: `returnTo=${encodeURIComponent(loginUrl)}`
        }).toString();
        return (
            <ApplicationFrame service={service}>
                <div className="sorry">
                    <h3>Hey {currentUser.name}!</h3>
                    <p>
                        You are currently logged in with {currentUser.authHost}/{currentUser.authName} and you trying to connect with {otherUser.authHost}/{otherUser.authName}, but this provider identity is already connected to your other account ({otherUser.name}).
                    </p>
                    <p>
                        Ideally, you would proceed with a single account and connect with both providers. To make a well considered decision, please review the contents and subscriptions of both accounts.

                    </p>
                    <p>
                        You can <a href={logoutUrl}>re-login with {otherUser.authHost}</a> to switch to your other account ({otherUser.name}). Feel free to disconnect the providers from the no longer required account and connect them with your main account afterwards.
                    </p>
                </div>
            </ApplicationFrame>
        );
    }
}

renderEntrypoint(ProceedWithAccount);
