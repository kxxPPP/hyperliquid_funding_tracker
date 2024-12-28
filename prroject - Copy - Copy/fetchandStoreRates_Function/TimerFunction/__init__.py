import datetime
import logging
import os
import requests
import json
from azure.data.tables import TableServiceClient, TableEntity
import azure.functions as func


def main(mytimer: func.TimerRequest) -> None:
    """
    Timer Trigger entry point. This function runs automatically based on your CRON schedule.
    """

    logging.info('Python Timer trigger function started.')

    # fetch apidata
    url = "https://api.hyperliquid.xyz/info"
    payload = {"type": "metaAndAssetCtxs"}
    headers = {"Content-Type": "application/json"}

    try:
        resp = requests.post(url, json=payload, headers=headers, timeout=30)
        resp.raise_for_status()
        data = resp.json()  
    except Exception as e:
        logging.error(f"Error fetching data from {url}: {e}")
        return

    try:
        universe_obj = data[0]  
        market_data = data[1]   

        universe_list = universe_obj.get("universe", [])
    except (IndexError, AttributeError, TypeError) as e:
        logging.error(f"Error parsing JSON response structure: {e}")
        return

    matched_data = []
    for i, asset in enumerate(universe_list):
        if i < len(market_data):
            market = market_data[i]
            matched_data.append({
                "name": asset["name"],
                "funding_rate": market["funding"],
                "maxLeverage": asset["maxLeverage"],
                "premium": market["premium"]
            })

    # azure table storage
    try:
        store_in_table(matched_data)
    except Exception as e:
        logging.error(f"Error storing data in table: {e}")
        return

    logging.info(f"Successfully stored {len(matched_data)} records in table.")


def store_in_table(data_list: list) -> None:
    """
    Stores each entry in the data list as a separate entity in Azure Table Storage.
    """
    conn_str = os.getenv("AZURE_STORAGE_CONNECTION_STRING")
    if not conn_str:
        raise ValueError("AZURE_STORAGE_CONNECTION_STRING is not set in environment.")

    service_client = TableServiceClient.from_connection_string(conn_str)
    table_name = "FundingRates"  
    table_client = service_client.get_table_client(table_name)
    try:
        table_client.create_table()
    except Exception as e:
        logging.info(f"create_table error (table might exist already): {e}")

    for record in data_list:
        partition_key = record["name"]  
        row_key = datetime.datetime.utcnow().strftime("%Y%m%d-%H%M%S-%f")  

        entity = TableEntity()
        entity["PartitionKey"] = partition_key
        entity["RowKey"] = row_key
        entity["name"] = record["name"]
        entity["funding_rate"] = record["funding_rate"]
        entity["maxLeverage"] = record["maxLeverage"]
        entity["premium"] = record["premium"]

        try:
            table_client.create_entity(entity=entity)
            logging.info(f"Inserted entity with RowKey={row_key}")
        except Exception as e:
            logging.error(f"Failed to insert entity for {record['name']}: {e}")
