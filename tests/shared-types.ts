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
