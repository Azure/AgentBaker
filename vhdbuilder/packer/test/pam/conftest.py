def pytest_configure(config):
    config.addinivalue_line(
        "markers", "user_data(name): mark test to run with user data"
    )

def pytest_addoption(parser):
    parser.addoption(
        "--fedramp",
        action="store_true",
        default=False,
        help="FedRAMP remediations have been applied",
    )
