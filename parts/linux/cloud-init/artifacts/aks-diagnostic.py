#! /usr/bin/env python3

"""
aks-diagnostic.py

This script uploads a diagnostic log file to an Azure Blob Storage container using a REST API call.
Usage:
    python aks-diagnostic.py <container> <storage_account_url> <sas_token> [file_path]

Arguments:
    container              : Name of the blob container.
    storage_account_url    : Full URL of the Azure storage account (e.g., https://<account>.blob.core.windows.net/).
    sas_token              : Shared Access Signature token for authentication.
    file_path (optional)   : Path to the log file to upload (default: /var/lib/waagent/logcollector/logs.zip).
"""
import requests
import os
import sys
import time
import argparse
from http import HTTPStatus
from urllib.parse import urlparse

# Configuration constants
MAX_RETRIES = 3
RETRY_DELAY = 2  # seconds
MAX_FILE_SIZE = 1024 * 1024 * 1024  # 1000MB
DEFAULT_LOG_PATH = "/var/lib/waagent/logcollector/logs.zip"

def parse_arguments():
    """
    Parse and validate command line arguments.
    
    Returns:
        argparse.Namespace: Parsed arguments
    """
    parser = argparse.ArgumentParser(
        description='Upload diagnostic log files to Azure Blob Storage',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
        Examples:
        python3 aks-diagnostic.py mycontainer https://mystorageaccount.blob.core.windows.net "sv=2020-08-04&ss=b&srt=co&sp=rwdlac&se=2023-01-01T00:00:00Z&st=2022-01-01T00:00:00Z&spr=https&sig=..."
        python3 aks-diagnostic.py mycontainer https://mystorageaccount.blob.core.windows.net "sv=2020-08-04&ss=b&srt=co&sp=rwdlac&se=2023-01-01T00:00:00Z&st=2022-01-01T00:00:00Z&spr=https&sig=..." /custom/path/logs.zip
        """
    )
    
    parser.add_argument(
        'container',
        help='Name of the blob container'
    )
    parser.add_argument(
        'storage_account_url',
        help='Full URL of the Azure storage account (e.g., https://<account>.blob.core.windows.net/)'
    )
    parser.add_argument(
        'sas_token',
        help='Shared Access Signature token for authentication'
    )
    parser.add_argument(
        'file_path',
        nargs='?',
        default=DEFAULT_LOG_PATH,
        help=f'Path to the log file to upload (default: {DEFAULT_LOG_PATH})'
    )
    
    return parser.parse_args()


def validate_arguments(args):
    """
    Validate parsed arguments.
    
    Args:
        args: Parsed arguments from argparse
        
    Returns:
        tuple: (container, storage_account_url, sas_token, file_path)
        
    Raises:
        SystemExit: If validation fails
    """
    # Validate storage account URL
    try:
        parsed_url = urlparse(args.storage_account_url)
        if not parsed_url.scheme or not parsed_url.netloc:
            print(f"Error: Invalid storage account URL: {args.storage_account_url}")
            sys.exit(1)
        if not parsed_url.scheme.startswith('http'):
            print(f"Error: Storage account URL must use HTTP or HTTPS: {args.storage_account_url}")
            sys.exit(1)
    except Exception as e:
        print(f"Error: Failed to parse storage account URL: {e}")
        sys.exit(1)
    
    # Validate file path
    if not os.path.isfile(args.file_path):
        print(f"Error: File not found: {args.file_path}")
        sys.exit(1)
        
    # Check file size
    try:
        file_size = os.path.getsize(args.file_path)
        if file_size > MAX_FILE_SIZE:
            print(f"Error: File size ({file_size} bytes) exceeds maximum allowed size ({MAX_FILE_SIZE} bytes)")
            sys.exit(1)
        if file_size == 0:
            print(f"Error: File is empty: {args.file_path}")
            sys.exit(1)
    except OSError as e:
        print(f"Error: Cannot access file {args.file_path}: {e}")
        sys.exit(1)
    
    # Validate container name (basic validation)
    if not args.container or not args.container.replace('-', '').replace('.', '').isalnum():
        print(f"Error: Invalid container name: {args.container}")
        sys.exit(1)
        
    # Validate SAS token (basic check)
    if not args.sas_token or 'sig=' not in args.sas_token:
        print("Error: Invalid SAS token format")
        sys.exit(1)
    
    return args.container, args.storage_account_url, args.sas_token, args.file_path


def handleArgs(args):
    if len(args) < 3:
        print("Usage: python aks-diagnostic.py <container> <storage_account_url> <sas_token> [file_path]")
        return None, None, None, None

    container = args[0]
    storage_account_url = args[1]
    sas_token = args[2]
    file_path = args[3] if len(args) > 3 else DEFAULT_LOG_PATH

    return container, storage_account_url, sas_token, file_path


def redact_sas_token(url):
    """
    Redact sensitive information from SAS token in URL for safe logging.
    
    Args:
        url (str): URL containing SAS token
        
    Returns:
        str: URL with SAS token signature redacted
    """
    if 'sig=' in url:
        parts = url.split('sig=')
        if len(parts) > 1:
            # Keep first few characters of signature for debugging, redact the rest
            sig_part = parts[1].split('&')[0]
            redacted_sig = sig_part[:4] + '***REDACTED***' if len(sig_part) > 4 else '***REDACTED***'
            remaining = '&'.join(parts[1].split('&')[1:]) if '&' in parts[1] else ''
            redacted_url = parts[0] + 'sig=' + redacted_sig
            if remaining:
                redacted_url += '&' + remaining
            return redacted_url
    return url


def upload_file_to_blob_rest(storage_account_url, sas_token, file_path):
    """
    Upload a file to Azure Blob Storage using REST API with retry logic.
    
    Args:
        storage_account_url (str): Base URL of the storage account
        sas_token (str): SAS token for authentication
        file_path (str): Path to the file to upload
        
    Returns:
        bool: True if upload successful, False otherwise
    """
    # Extract file name to use as blob name
    blob_name = os.path.basename(file_path)
    
    # Construct the full URL for the blob including SAS token
    blob_url = f"{storage_account_url.rstrip('/')}/{blob_name}?{sas_token}"
    
    # Get file size for logging
    file_size = os.path.getsize(file_path)
    
    print(f"Uploading file: {file_path}")
    print(f"File size: {file_size} bytes")
    print(f"Blob name: {blob_name}")
    print(f"Request URL: {redact_sas_token(blob_url)}")
    
    for attempt in range(MAX_RETRIES):
        try:
            print(f"Upload attempt {attempt + 1}/{MAX_RETRIES}")
            
            # Read the file content
            with open(file_path, "rb") as file_data:
                headers = {
                    "x-ms-blob-type": "BlockBlob",
                    "Content-Length": str(file_size)
                }
                
                # Make the request with timeout
                response = requests.put(
                    blob_url, 
                    headers=headers, 
                    data=file_data,
                    timeout=(30, 300)  # (connect_timeout, read_timeout)
                )
                
                if response.status_code == HTTPStatus.CREATED:
                    print(f"Upload successful: {blob_name}")
                    return True
                elif response.status_code in [HTTPStatus.TOO_MANY_REQUESTS, 
                                            HTTPStatus.INTERNAL_SERVER_ERROR,
                                            HTTPStatus.BAD_GATEWAY,
                                            HTTPStatus.SERVICE_UNAVAILABLE,
                                            HTTPStatus.GATEWAY_TIMEOUT]:
                    # Retryable errors
                    print(f"Retryable error (attempt {attempt + 1}): {response.status_code} - {response.reason}")
                    if attempt < MAX_RETRIES - 1:
                        wait_time = RETRY_DELAY * (2 ** attempt)  # Exponential backoff
                        print(f"Waiting {wait_time} seconds before retry...")
                        time.sleep(wait_time)
                        continue
                else:
                    # Non-retryable errors
                    print(f"Upload failed with non-retryable error: {response.status_code} - {response.reason}")
                    try:
                        error_body = response.text
                        if error_body:
                            print(f"Error response body: {error_body}")
                    except Exception:
                        print("Could not read error response body")
                    return False
                    
        except requests.exceptions.Timeout:
            print(f"Request timeout (attempt {attempt + 1})")
            if attempt < MAX_RETRIES - 1:
                wait_time = RETRY_DELAY * (2 ** attempt)
                print(f"Waiting {wait_time} seconds before retry...")
                time.sleep(wait_time)
            else:
                print("Upload failed: Request timeout after all retries")
                return False
                
        except requests.exceptions.ConnectionError as e:
            print(f"Connection error (attempt {attempt + 1}): {e}")
            if attempt < MAX_RETRIES - 1:
                wait_time = RETRY_DELAY * (2 ** attempt)
                print(f"Waiting {wait_time} seconds before retry...")
                time.sleep(wait_time)
            else:
                print("Upload failed: Connection error after all retries")
                return False
                
        except FileNotFoundError:
            print(f"File not found: {file_path}")
            return False
            
        except PermissionError:
            print(f"Permission denied reading file: {file_path}")
            return False
            
        except OSError as e:
            print(f"OS error reading file {file_path}: {e}")
            return False
            
        except Exception as e:
            print(f"Unexpected error (attempt {attempt + 1}): {e}")
            if attempt < MAX_RETRIES - 1:
                wait_time = RETRY_DELAY * (2 ** attempt)
                print(f"Waiting {wait_time} seconds before retry...")
                time.sleep(wait_time)
            else:
                print("Upload failed: Unexpected error after all retries")
                return False
    
    print(f"Upload failed after {MAX_RETRIES} attempts")
    return False

# Example usage
if __name__ == "__main__":
    print("AKS Diagnostic Log Uploader - Starting...")
    
    try:
        # Parse and validate arguments
        args = parse_arguments()
        container, storage_account_url, sas_token, file_path = validate_arguments(args)
        
        print(f"Container: {container}")
        print(f"Storage Account URL: {storage_account_url}")
        print(f"File path: {file_path}")
        #  Redact during print to protect sensitive info
        print(f"SAS Token: {redact_sas_token(sas_token)}")
        
        # Ensure exactly one slash between storage_account_url and container
        full_url = f"{storage_account_url.rstrip('/')}/{container.lstrip('/')}"
        print(f"Full container URL: {full_url}")
        
        # Attempt upload
        success = upload_file_to_blob_rest(full_url, sas_token, file_path)
        
        if success:
            print("AKS Diagnostic Log Uploader - Completed successfully")
            sys.exit(0)
        else:
            print("AKS Diagnostic Log Uploader - Failed")
            sys.exit(1)
        
    except Exception as e:
        print(f"Unexpected error in main: {e}")
        sys.exit(2)

