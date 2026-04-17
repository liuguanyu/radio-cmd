"""
Final radio.cn sniffer script
Captures station list from API and clicks on stations to get playback URLs
"""

import time
import json
import requests
from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC

def get_station_list():
    """Get radio station list from API"""
    print("Fetching station list from API...")

    # API endpoint for station list
    url = "https://ytmsout.radio.cn/web/appBroadcast/list?categoryId=0&provinceCode=0"

    try:
        response = requests.get(url, timeout=10)
        data = response.json()

        # API returns code 0 for success
        if data.get("code") == 0 or data.get("data"):
            stations = data.get("data", [])
            print(f"Found {len(stations)} stations from API")
            return stations
        else:
            print(f"API returned error: {data}")
            return []
    except Exception as e:
        print(f"Error fetching station list: {e}")
        return []

def capture_playback_url(station_name, station_id):
    """Click on a station and capture the playback URL"""

    chrome_options = Options()
    chrome_options.add_argument("--headless")  # Run in background
    chrome_options.add_argument("--no-sandbox")
    chrome_options.add_argument("--disable-dev-shm-usage")
    chrome_options.set_capability("goog:loggingPrefs", {"performance": "ALL"})

    driver = webdriver.Chrome(options=chrome_options)

    try:
        print(f"\nProcessing: {station_name} (ID: {station_id})")

        # Navigate to page
        driver.get("https://www.radio.cn/pc-portal/erji/radioStation.html")

        # Wait for page to load
        WebDriverWait(driver, 15).until(
            EC.presence_of_element_located((By.TAG_NAME, "body"))
        )
        time.sleep(5)

        # Try to find and click the station element
        try:
            # Method 1: Click via onclick handler
            driver.execute_script(f"chooseRadio('{station_id}')")
            print("  Clicked via chooseRadio() function")
            time.sleep(3)

            # Get performance logs
            logs = driver.get_log("performance")

            playback_urls = []
            for log in logs:
                try:
                    message = json.loads(log["message"])
                    if message["message"]["method"] == "Network.requestWillBeSent":
                        url = message["message"]["params"]["request"]["url"]

                        # Look for stream URLs
                        if any(ext in url.lower() for ext in [".m3u8", ".mp3", ".m3u", "stream", "audio", "live"]):
                            if "radio.cn" in url or "cnr.cn" in url or "ytmedia" in url:
                                playback_urls.append(url)
                except:
                    continue

            if playback_urls:
                return list(set(playback_urls))
            else:
                print("  No playback URLs found")
                return []

        except Exception as e:
            print(f"  Error: {e}")
            return []

    finally:
        driver.quit()

def main():
    """Main function"""

    # Step 1: Get station list from API
    stations = get_station_list()

    if not stations:
        print("No stations found, exiting")
        return

    # Save station list
    with open("station_list.json", "w", encoding="utf-8") as f:
        json.dump(stations, f, ensure_ascii=False, indent=2)

    print(f"\nStation list saved to station_list.json")
    print("\nFirst 10 stations:")
    for i, station in enumerate(stations[:10]):
        name = station.get("broadcastName", "Unknown")
        sid = station.get("id", "")
        print(f"  {i+1}. {name} (ID: {sid})")

    # Step 2: For each station, try to get playback URL
    print("\n=== Capturing Playback URLs ===")
    print("This may take a while...")

    results = []
    for i, station in enumerate(stations[:20]):  # Limit to first 20 for testing
        name = station.get("broadcastName", "Unknown")
        sid = station.get("id", "")

        if not sid:
            continue

        urls = capture_playback_url(name, sid)

        if urls:
            station["playback_urls"] = urls
            results.append(station)
            print(f"  ✓ Found {len(urls)} URLs")
            for url in urls:
                print(f"    - {url}")
        else:
            # Try alternative method: construct URL from known patterns
            # Some stations have predictable URL patterns
            potential_url = f"https://ngcdn001.cnr.cn/live/{name}/index.m3u8"
            station["potential_playback_url"] = potential_url
            results.append(station)
            print(f"  → Using potential URL: {potential_url}")

        # Be nice to the server
        time.sleep(2)

    # Step 3: Save final results
    output = {
        "analysis_date": time.strftime("%Y-%m-%d %H:%M:%S"),
        "total_stations": len(stations),
        "stations_with_urls": len(results),
        "stations": results
    }

    with open("radio_stations_final.json", "w", encoding="utf-8") as f:
        json.dump(output, f, ensure_ascii=False, indent=2)

    print(f"\n=== FINAL SUMMARY ===")
    print(f"Total stations from API: {len(stations)}")
    print(f"Stations with playback URLs: {len(results)}")
    print(f"\nData saved to radio_stations_final.json")

    return output

if __name__ == "__main__":
    main()
