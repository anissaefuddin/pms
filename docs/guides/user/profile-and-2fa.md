# Profile, password and two-factor authentication

## Profile
Account menu → **Settings** → **Profile**. Fields: First name, Last name, Company, Job title, Country code, Location, Department, Phone, **Time zone**, **Language**, **Date format**, **Time format**, **Week starts on**, **Default task duration (days)**, **Default task effort (minutes)**. **Appearance**: **Density** (Compact/Comfortable), **Theme** (Light/Dark/System), **Color theme** (Blue/Teal/Violet/Green), **Send comment shortcut**. Also **Email notifications** ([Notifications](notifications-and-email.md)) and **Sidebar** entries (My to-do, Request forms, Stream, Timesheets). **Save profile**.

## Sign in
Email + password (**Log in**), **Forgot password?**, **Email me a sign-in code** (six digits, once, valid 10 minutes), or **Sign in with {provider}** if SSO is configured. New users accept an invitation link (**Get started**).

## Change password
**Security** (“Sign-in & security”) → **Change password**. Other sessions are signed out. Minimum 8 characters.

## Two-factor authentication
1. **Security** → **Set up two-factor authentication** (needs the server key set by an admin; otherwise “not available on this server yet”).
2. Scan the QR code with an authenticator app (or **Copy key** for manual entry).
3. Enter the 6-digit **Code from your app** → **Turn on**. Other sessions are signed out.
4. **Save backup codes** (**Copy codes** / **Download .txt**; shown once), then **I saved these codes**.
5. At sign-in enter the **Authentication code** (app code or a backup code) → **Verify**.
- **Generate new backup codes** and **Turn off two-factor authentication** both need your password plus an app/backup code.

## Limitations
Language list shows English only. Lost phone and no backup codes: ask an administrator to reset 2FA.
