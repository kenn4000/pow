# Android device playtest

This is an installed Capacitor Android application (debug APK), not a browser tab. It loads the existing HTTPS game deployment inside the native WebView. It does not bundle the React game yet; changes to the game must be deployed to the configured URL before they appear in the app. It needs internet and a running backend. This is a development-only remote-server configuration, not a final Play Store release.

## Build on Windows
Install Node.js 22, Android Studio, Android SDK Platform 35, and JDK 21. Set ANDROID_HOME to your SDK directory and JAVA_HOME to the JDK 21 directory. In PowerShell from the repository root:

```powershell
cd mobile
npm install
npm run android:init
npm run android:sync
cd android
.\gradlew.bat assembleDebug
```

Run android:init only the first time. The APK is android/app/build/outputs/apk/debug/app-debug.apk relative to mobile. Copy it to your phone, open it and grant the file manager permission to install unknown apps if prompted. Alternatively enable USB debugging, connect your phone and install with adb install -r. Keep your normal Android security settings enabled; only authorize the installer you trust.

To use Android Studio, from mobile run npm run android:open after initialization.

## CI
The Android playtest workflow builds an unsigned-for-release, debug-signed APK artifact; download and extract the ZIP from a successful Actions run. The debug key is ephemeral on hosted runners, so a later build may require uninstalling the previous debug app. No production signing keys are required.

## Server
The default URL is the hosted deployment documented in the root README. To test the mobile branch, deploy its web frontend/backend first to that URL or a separate trusted HTTPS staging URL and change server.url in capacitor.config.json before syncing and rebuilding. Do not use localhost on a physical phone. Do not enable cleartext or mix untrusted origins. Do not enable passwordless development login on a reachable host.

## Scope
Check sign-in, saved games, every card mode, safe areas, Android back, background/resume and network loss. This prototype has not yet been tested on a device. Default generated Capacitor icon and platform lifecycle behavior are used; custom branding, offline error UX, native back behavior, bundled client and production signing are follow-up work.

## Build against your Windows PC over Wi-Fi

1. On your PC run `docker compose up -d`, then confirm `http://localhost:8088/prisoner-of-war/` works.
2. Run `ipconfig` and find your active adapter's IPv4 address (usually `192.168.x.x`). Phone and PC must be on the same Wi-Fi; permit inbound TCP 8088 in Windows Firewall for the private network only.
3. On GitHub Actions, choose **Android playtest APK** → **Run workflow**, select branch `android-playtest`, and enter `http://YOUR-PC-IP:8088/prisoner-of-war/` in **server_url**.
4. Download the resulting APK artifact and install on Android. Rebuild if your PC's LAN IP changes.

HTTP is deliberately permitted only for RFC1918 private IPv4 addresses. Do not use localhost (which would refer to the phone), or expose port 8088 publicly. This debug build uses Capacitor's remote-server mode and displays whatever frontend is served by your PC. To pick up frontend source changes, rebuild the local Docker images using `docker compose up --build -d`.

## Startup artwork

The proposed studio and game loading artwork is supplied separately as a ZIP because the repository editing integration cannot upload binary PNGs. Add the two PNGs under `mobile/assets/` and commit them to this branch. They are source assets only until an Android native splash/loading implementation is added; merely adding them does not display them in the APK.
