def pytest_configure(config):
    config.addinivalue_line(
        "markers", "user_data(name): mark test to run with user data"
    )