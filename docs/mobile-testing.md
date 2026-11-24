# Mobile playtest

Deploy the existing frontend and backend together using the normal deployment process. Serve over HTTPS at the configured Vite base (/prisoner-of-war/). Open the deployed URL in Safari on iOS or Chrome on Android. Use Add to Home Screen / Install app if offered. The game requires a live backend; no offline gameplay or cached authenticated API responses are introduced.

## Device matrix
- 320x568 and 360x640: smallest supported portrait sizes
- 390x844 and 430x932: modern iPhones, including safe areas
- 412x915: Android portrait
- Landscape and tablet: verify scrolling and no trapped controls

## Manual acceptance
- Sign in, create account, keyboard visibility, sign out, account navigation.
- Dashboard: footer, currencies, offers, scrolling, every feature tab.
- Every battle variant: card selection, selection feedback, action buttons, turn transitions, results and resume.
- Campaigns, shop, heroes, skills, card catalog, multiplayer queue and reconnection.
- Modals: close/confirm, scroll, no clipping behind browser chrome or notch.
- Test actual taps on iOS Safari and Android Chrome, including standalone mode. Confirm backend session persistence and refresh.
- Check desktop regression and run npm ci && npm run build in frontend.

This checklist is not a report of completed testing. Native APK/IPA packaging and store signing are separate follow-up work.
