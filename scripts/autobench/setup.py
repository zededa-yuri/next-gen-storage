from setuptools import setup

setup(
    name='autobench',
    version='0.0.1',
    install_requires=[
        'Click',
        'netmiko',
        'parallel-ssh',
    ],

    entry_points={
        'console_scripts': [
            'autobench = bench:cli',
        ],
    },
)
