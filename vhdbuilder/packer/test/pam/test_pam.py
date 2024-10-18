import os
import pwd
import random
import string
import sys

import pexpect
import pytest


class BashSpawn(pexpect.spawn):
    """
    Extension of pexpect.spawn to handle bash, login and password prompts
    """
    def __init(self, command, args=[]):
        super().__init__(command, args, logfile=sys.stdout, timeout=1)

    def expect_login_prompt(self, user):
        print("\n-----Expecting login prompt")
        self.expect([r"[Ll]ogin: "], timeout=1)
        print("\n-----Sending username")
        self.sendline(user)

    def expect_password_prompt(self, pw):
        print("\n-----Expecting password prompt")
        self.expect([r"[Pp]assword: "], timeout=1)
        print("\n-----Sending password")
        self.waitnoecho()
        self.sendline(pw)

    def expect_bash_prompt(self):
        print("\n-----Expecting bash prompt")
        self.expect([r"\$ "], timeout=1)
        print("\n-----Found bash prompt")


def check_user_exists(username):
    """
    Check if the specified user exists

    Input Arguments:
      username (str): The name of the user to check

    Returns:
      True if the user exists, False if not
    """
    return any(user[0] == username for user in pwd.getpwall())


def gen_pw(length=24, use_lowercase=True, use_uppercase=True, use_numbers=True,
           use_specials=True):
    """
    Generate a random password

    Input Arguments:
      length (int): The length of the password to generate
      use_lowercase (bool): Whether to use lowercase characters
      use_uppercase (bool): Whether to use uppercase characters
      use_numbers (bool): Whether to use numbers
      use_specials (bool): Whether to use special characters

    Returns:
      The generated password
    """
    print("\n-----Generating password")
    allowed_charsets = []
    pw = ''

    # Ensure at least one of each type of character requested character type
    # is used
    if use_lowercase:
        allowed_charsets += [string.ascii_lowercase]
        pw += random.choice(string.ascii_lowercase)
    if use_uppercase:
        allowed_charsets += [string.ascii_uppercase]
        pw += random.choice(string.ascii_uppercase)
    if use_numbers:
        allowed_charsets += [string.digits]
        pw += random.choice(string.digits)
    if use_specials:
        allowed_charsets += ["!@#"]
        pw += random.choice("!@#")
    if len(pw) > length:
        raise Exception("Password length is greater than specified length")

    # Add random characters from the character sets until the password is the
    # specified length, making sure we don't have enough repeated characters
    # or character classes to violate the password policy.
    for i in range(length - len(pw)):
        pw += random.choice(allowed_charsets[i % len(allowed_charsets)])

    return pw


def set_password(username, password):
    """
    Set the password for the specified user

    Input Arguments:
      username (str): The name of the user to set the password for;
      password (str): The password to set for the user
    """
    print("\n=== Setting password for " + username + " to " + password)
    child = BashSpawn(f"sudo passwd {username}", encoding='utf-8',
                      logfile=sys.stdout)
    try:
        child.expect_password_prompt(password)
        child.expect(["Retype new password:"])
        print("\n-------Confirming password")
        child.sendline(password)

        child.expect([pexpect.EOF])
    except Exception as e:
        print("\n-----Failed to set password: " + str(e))
        pytest.fail("Failed to set password")
    finally:
        child.close()
    print("\n-----Initial password set successfully")


class User:
    """
    User Class to create a user with a random password and delete the user
    """
    def __init__(self, name):
        """"
        User Class Constructor to init an object with a random password

        Input Arguments:
          name (str): The name of the user to create
        """
        self.name = name
        self.pw = gen_pw()

    def create(self):
        """
        Create the user in the OS and set the password

        Returns:
          the password that was set for the user

        Raises:
          Exception: If the user is not created successfully or the password is
          not set successfully
        """
        print(f"\n=== Adding user {self.name} with password {self.pw}")
        child = BashSpawn("sudo useradd -M -s /bin/bash -K PASS_MIN_DAYS=0 " +
                          f"{self.name}",
                          encoding='utf-8')
        try:
            child.expect([pexpect.EOF])

            if not check_user_exists(self.name):
                raise Exception("useradd failed")

            set_password(self.name, self.pw)
        except Exception as e:
            print("\n-----Failed to add user: " + str(e))
            pytest.fail("Failed to add user")
        finally:
            child.close()

        print("\n-----User added successfully")
        return self.pw

    def delete(self):
        """
        Delete the user and remove the faillock file

        Raises:
          Exception: If the user is not deleted successfully or the faillock
          file is not removed successfully
        """

        print(f"\n=== Deleting user {self.name}")
        child = BashSpawn(f"sudo userdel -f {self.name}", encoding='utf-8')
        try:
            child.expect([pexpect.EOF])

            if check_user_exists(self.name):
                raise Exception(f"userdel failed for {self.name}")

            print(f"\n-----User {self.name} deleted")

        except Exception as e:
            print("\n-----Failed to delete user: " + str(e))
            pytest.fail("Failed to delete user")
        finally:
            child.close()

        print(f"\n-----Removing faillock file for {self.name}")
        child = BashSpawn(f"sudo rm -f /var/run/faillock/{self.name}",
                          encoding='utf-8')
        try:
            child.expect([pexpect.EOF])

        except Exception as e:
            print("\n-----Failed to delete user: " + str(e))
            pytest.fail("Failed to delete user")
        finally:
            child.close()


@pytest.fixture
def create_user(request):
    """
    Create a user then delete it when the test is complete

    Create a user, yeild to run the test, then delete the user when the
    test is complete. The username is specified by the user_data marker.
    """
    marker = request.node.get_closest_marker("user_data")
    if marker is None:
        raise Exception("No user_data marker found")
    user = User(marker.args[0])
    user.create()
    yield user
    user.delete()

@pytest.fixture
def get_deny_count(request):
    option = request.config.getoption("--fedramp")
    if option:
        return 3
    else:
        return 5

def login(user, pw=None):
    """
    Login as user with password pw

    Login as user with password pw. If pw is not specified, the password
    contained in the user object is used.

    Keyword arguments:
      user -- User object
      pw -- Password to use for login (default None)

    Returns:
      True if login was successful, False otherwise
    """
    login_successful = True
    if pw is None:
        pw = user.pw
    print("\n=== Logging in as " + user.name)
    child = BashSpawn("sudo login", encoding='utf-8')
    try:
        child.expect_login_prompt(user.name)
        child.expect_password_prompt(pw)

        i = child.expect([r"\$", r"Login incorrect", r"Login failed"])
        if i == 1 or i == 2:
            print(f"\n-----Login as {user.name} failed")
            raise Exception(f"Login as {user.name} failed")
        else:
            print(f"\n-----Login as {user.name} successful")
            child.sendline("whoami")
            child.expect(user.name)

    except Exception as e:
        print("\n-----Failed to login: " + str(e))
        login_successful = False
    finally:
        child.close()

    return login_successful


def change_password(user, new_pw):
    """
    Change the password of a user

    Keyword arguments:
      user -- User object;
      new_pw -- new password string

    Returns:
      True if password change was successful, False otherwise
    """
    print(f"\n=== Changing password for {user.name} to {new_pw}")
    result = False
    child = BashSpawn(f"sudo login {user.name}", encoding='utf-8',
                      logfile=sys.stdout, timeout=1)
    try:
        child.expect_password_prompt(user.pw)
        child.expect([r"\$"])
        child.sendline("whoami")
        child.expect(user.name)
        print("\n-----logged in as " + user.name)
        child.sendline("passwd")
        print("\n-----sending current password")
        child.expect_password_prompt(user.pw)
        print("\n-----changing password")
        child.expect(["[Nn]ew.*password:"])
        child.sendline(new_pw)
        print("\n-----sent new password")
        i = child.expect([r"BAD PASSWORD: (.*?)(?=\n)",
                          "Retype new.*password:"])
        print(f"\n-----matched {i}")
        if i == 0:
            m = child.match.group(1)
            print("\n-----Bad password: " + m)
            result = False
        elif i == 1:
            print("\n-----retype new password")
            child.sendline(new_pw)
            i = child.expect([r"passwd:.*updated successfully",
                              r"already used"])
            if i == 0:
                print("\n-----expecting EOF")
                child.expect([r"\$ ", pexpect.EOF])
                user.pw = new_pw
                print(f"\n-----Password for {user.name} changed to {new_pw}")
                result = True
            elif i == 1:
                print("\n-----Password already used")
                result = False
    except Exception as e:
        print("\n-----Exception: Failed to change password: " + str(e))
        result = False
    finally:
        child.close()

    return result


def change_password_w_retries(user, bad_pw):
    """Change password, retrying until retry limit is reached.

    Given a password that should be rejected, repeatedly attempt to
    change the password for the given user.  If the password is rejected,
    retry until the retry limit is reached, or the password is successfully
    changed.

    Keyword arguments:
    user -- User object;
    bad_pw -- invalid password to use

    Returns:
    Tuple of (complete, count) where:
      complete is True if retry limit was reached, False otherwise;
      count is the number of attempts made
    """

    print(f"\n===Attempting to change password for {user.name} to invalid " +
          f"pw '{bad_pw}'")
    child = BashSpawn(f"sudo login {user.name}", encoding='utf-8')
    count = 0
    try:
        child.expect_password_prompt(user.pw)
        child.expect([r"\$"])

        # verify successful login
        child.sendline("whoami")
        child.expect(user.name)
        print("\n-----logged in as " + user.name)

        # attempt to change password
        child.sendline("passwd")
        print("\n-----sending current password")
        child.expect_password_prompt(user.pw)

        done = False
        while not done:
            i = child.expect(["[Nn]ew.*password:",
                              "already used",
                              "password unchanged\r\n"])
            if i == 0 or i == 1:
                count += 1
                print(f"\n-----sending new password (attempt {count})")
                child.sendline(bad_pw)
            elif i == 2:
                print("\n-----Password unchanged!!")
                done = True

        child.expect([r"\$ ", pexpect.EOF], timeout=1)

    except Exception as e:
        print("\n-----Unexpected error while attempting to change password: "
              + str(e))
        return (False, count)
    finally:
        child.close()

    return (True, count)


def test_faillock_dir_exists():
    assert os.path.exists("/var/log/faillock")


@pytest.mark.user_data("testuser1")
def test_user_can_login(create_user):
    assert login(create_user)


@pytest.mark.user_data("testuser2")
def test_user_auth_fails_w_bad_password(create_user):
    assert not login(create_user, gen_pw())


@pytest.mark.user_data("testuser3")
def test_user_auth_locks_after_deny_count_failures(create_user, get_deny_count):
    user = create_user

    # fail five times, which should trigger the faillock
    for i in range(get_deny_count):
        assert not login(user, "invalid"), f"Login attempt {i} with \
            invalid password succeeded"

    # try to login again with the correct password
    # if the faillock is working, this should fail
    assert not login(user)


@pytest.mark.user_data("testuser4")
def test_user_auth_faillock_resets_after_success(create_user, get_deny_count):
    user = create_user

    # use an invalid password so we fail enough times to put the account
    # into one try away from a faillock
    for i in range(get_deny_count - 1):
        assert not login(user, "invalid"), f"Login attempt {i} with invalid \
            password succeeded"

    # log in successfully which should reset the faillock
    assert login(user), "Login with valid password should have succeeded"

    # fail once more which should lock the account if the reset is not working
    assert not login(user, "invalid"), "Login with invalid password should \
        have failed"

    # try to login again with the correct password
    # if the faillock reset as expected, this should succeed
    assert login(user)


@pytest.mark.user_data("testuser5")
def test_pw_quality_requires_digit(create_user):
    assert not change_password(create_user, gen_pw(use_numbers=False))


@pytest.mark.user_data("testuser6")
def test_pw_quality_requires_uppercase(create_user):
    assert not change_password(create_user, gen_pw(use_uppercase=False))


@pytest.mark.user_data("testuser7")
def test_pw_quality_requires_lowercase(create_user):
    assert not change_password(create_user, gen_pw(use_lowercase=False))


@pytest.mark.user_data("testuser8")
def test_pw_quality_requires_non_alphanum(create_user):
    assert not change_password(create_user, gen_pw(use_specials=False))


@pytest.mark.user_data("testuser9")
def test_pw_quality_requires_min_len(create_user):
    assert not change_password(create_user, gen_pw(length=11))


@pytest.mark.user_data("testuser10")
def test_pw_quality_allows_3_retries(create_user):
    assert change_password_w_retries(create_user, "invalid") == (True, 3)


@pytest.mark.user_data("testuser11")
def test_user_can_change_password(create_user):
    assert change_password(create_user, gen_pw())


@pytest.mark.user_data("testuser12")
def test_pw_history_reuse_blocked(create_user):
    user = create_user
    pw_orig = user.pw

    # change the password four times so that we now have five passwords
    # worth of history
    for i in range(4):
        assert change_password(user, gen_pw()), f"Failed to change \
            password on attempt {i}"

    # try to change the password to the first one in the history
    assert not change_password(user, pw_orig)


if __name__ == "__main__":
    pytest.main(["-s", "-v", "pam_test.py"])
