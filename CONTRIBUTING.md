# Contributing

## Signing AgentBaker commits

### Steps to enable signature on commits
This is an example that has been tested on WSL Ubuntu 22.04 and Ubuntu 22.04. For Mac and Windows powershell it should be similar.

1. List an existing key on your local machine. `gpg --list-secret-keys --keyid-format=long`
   - If it's empty or if you want to use a new one, run `gpg --default-new-key-algo rsa4096 --gen-key`

2. Create a PGP Public key by running this command
   `gpg --armor --export <GPG Key ID>` where the GPG Key ID is the one you got from step 1.

   For more info, check step 3 in this doc. [Associating an email with your GPG key - GitHub Docs](https://docs.github.com/en/authentication/managing-commit-signature-verification/associating-an-email-with-your-gpg-key)

3. Finally add the public key to github by following this doc. https://docs.github.com/en/authentication/managing-commit-signature-verification/adding-a-gpg-key-to-your-github-account#adding-a-gpg-key

4. Re-do the commit with the correct command. `git commit -S -m "YOUR_COMMIT_MESSAGE"` and it should work now.

   - If you encounter error `gpg: signing failed: Inappropriate ioctl for device`, follow the below
   ``` git config --global gpg.program gpg
   git config --global commit.gpgsign true
   git config --global gpg.passphrase "<the passphrase you set in step 1 if you created a new one>"
   echo "use-agent" >> ~/.gnupg/gpg.conf
   echo "pinentry-mode loopback" >> ~/.gnupg/gpg.conf
   echo "allow-loopback-pinentry" >> ~/.gnupg/gpg-agent.conf
   gpgconf --kill gpg-agent
   gpgconf --launch gpg-agent
   ```
5. (Optional but recommended) Run `git config commit.gpgsign true` to config it to always sign with gpg in this repo.

Note: if you have previously pushed unsigned commit, you can try the following.

- run `git commit --amend -s`. You should see no errors.
    - `git rebase HEAD~<number of your commits on branch> --signoff` for multiple commits
- run `git push --force`. This should overwrite your previous commit with the new signed commit on remote branch/PR.

### More reference
- https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits

- https://docs.github.com/en/authentication/managing-commit-signature-verification/associating-an-email-with-your-gpg-key

- https://docs.github.com/en/authentication/managing-commit-signature-verification/adding-a-gpg-key-to-your-github-account#adding-a-gpg-key
