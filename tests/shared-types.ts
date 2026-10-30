import type {AppManifest,ApiErrorEnvelope,LegacyErrorEnvelope,AccountProfile,FileResource,FileAcl,Job,UploadSession,AuthorizationCheck,PlatformEvent} from '@minhanos/shared-types'
import type {Profile} from '@minhanos/app-sdk'
type IsAny<T> = 0 extends (1 & T) ? true : false
const checkedProfile: IsAny<Profile> = false
const profile: AccountProfile = {locale:'vi-VN',theme:'system',timeZone:'UTC'}
const sdkProfile: Profile = profile
// @ts-expect-error Unsupported locales must not widen to any/string.
const badLocale: Profile = {locale:'unsupported',theme:'light',timeZone:'UTC'}
// @ts-expect-error The generated job state must remain a closed union.
const badState: Job['state'] = 'paused'
type PublicContracts = [AppManifest,ApiErrorEnvelope,LegacyErrorEnvelope,FileResource,FileAcl,Job,UploadSession,AuthorizationCheck,PlatformEvent]
void checkedProfile;void sdkProfile;void badLocale;void badState

import type {IdentityUser,IdentityRole,IdentitySession} from '@minhanos/shared-types'
import {createIdentityClient} from '@minhanos/api-client/identity'
const identityClient=createIdentityClient()
// @ts-expect-error Role assignment is not a permitted profile patch.
void identityClient.identityPatchUser({userId:'id',ifMatch:'v1',body:{role:'admin'}})
// @ts-expect-error Idempotency is required for user creation.
void identityClient.identityCreateUser({body:{username:'new',displayName:'New',password:'not-a-real-secret'}})
// @ts-expect-error User status is a closed union.
const unsupportedStatus: IdentityUser['status']='locked-by-client'
type IdentityTypes=[IdentityUser,IdentityRole,IdentitySession]
void unsupportedStatus

void identityClient.identityListUsers()

void identityClient.identityReauthenticate({body:{password:'not-a-real-secret'}})
void identityClient.identitySetUserRole({userId:'id',ifMatch:'"1"',idempotencyKey:'typed-example',body:{role:'user-admin'}})
// @ts-expect-error Host operator is deliberately not a web role.
void identityClient.identitySetUserRole({userId:'id',ifMatch:'"1"',idempotencyKey:'typed-example',body:{role:'host-root'}})
