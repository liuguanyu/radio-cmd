"""
Radio.cn website sniffer script
Uses ChromeDriver to capture API requests for radio station data and playback URLs
"""

import time
import json
import os
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.common.desired_capabilities import DesiredCapabilities

# Set up Chrome options
chrome_options = Options()
chrome_options.add_argument("--headless")  # Run in background
chrome_options.add_argument("--no-sandbox")
chrome_options.add_argument("--disable-dev-shm-usage")
chrome_options.add_argument("--disable-gpu")
chrome_options.add_argument("--window-size=1920,1080")

# Enable network logging
# This captures all network requests
chrome_options.set_capability("goog:loggingPrefs", {"performance": "ALL"})

def capture_radio_data():
    """Capture radio station data and playback URLs from radio.cn"""
    driver = webdriver.Chrome(options=chrome_options)

    try:
        # Navigate to the radio station page
        driver.get("https://www.radio.cn/pc-portal/erji/radioStation.html")

        # Wait for page to load
        WebDriverWait(driver, 10).until(
            EC.presence_of_element_located((By.CLASS_NAME, "radio-station-list"))
        )

        # Wait a bit longer for JS to execute and load data
        time.sleep(5)

        # Scroll to trigger lazy loading if needed
        driver.execute_script("window.scrollTo(0, document.body.scrollHeight);")
        time.sleep(3)

        # Get performance logs
        logs = driver.get_log("performance")

        # Store found requests
        radio_requests = []
        playback_urls = []
        station_list = []

        # Parse logs for relevant network requests
        for log in logs:
            message = json.loads(log["message"])

            # Look for network requests
            if message["message"]["method"] == "Network.requestWillBeSent":
                url = message["message"]["params"]["request"]["url"]

                # Look for API endpoints that might contain station data
                if "radio.cn" in url and ("station" in url.lower() or "list" in url.lower() or "broadcast" in url.lower()):
                    radio_requests.append({
                        "type": "station_api",
                        "url": url,
                        "timestamp": message["message"]["params"]["timestamp"]
                    })

                # Look for audio playback URLs (likely .m3u8 or .mp3)
                if "radio.cn" in url and (".m3u8" in url or ".mp3" in url or "audio" in url or "stream" in url):
                    playback_urls.append({
                        "type": "playback_url",
                        "url": url,
                        "timestamp": message["message"]["params"]["timestamp"]
                    })

        # Extract station names from DOM
        stations = driver.find_elements(By.CSS_SELECTOR, ".station-item .station-name")
        for station in stations:
            station_name = station.text.strip()
            if station_name:
                station_list.append(station_name)

        # Also check for station data in other possible elements
        station_elements = driver.find_elements(By.CSS_SELECTOR, ".station-item")
        for elem in station_elements:
            try:
                name_elem = elem.find_element(By.CLASS_NAME, "station-name")
                name = name_elem.text.strip()
                if name:
                    # Try to find the station ID from data attributes
                    station_id = elem.get_attribute("data-station-id") or elem.get_attribute("id")
                    station_list.append({
                        "name": name,
                        "id": station_id
                    })
            except:
                continue

        # Return all captured data
        result = {
            "stations": station_list,
            "radio_api_requests": radio_requests,
            "playback_urls": playback_urls,
            "total_logs": len(logs)
        }

        return result

    finally:
        driver.quit()

if __name__ == "__main__":
    print("Starting radio.cn sniffer...")
    data = capture_radio_data()

    # Save results to file
    with open("radio_data.json", "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)

    print(f"Captured {len(data['stations'])} stations")
    print(f"Found {len(data['radio_api_requests'])} radio API requests")
    print(f"Found {len(data['playback_urls'])} playback URLs")
    print("Results saved to radio_data.json")

    # Print summary
    print("\n=== Station List ===")
    for station in data['stations']:
        if isinstance(station, str):
            print(f"- {station}")
        else:
            print(f"- {station['name']} (ID: {station['id']})")

    print("\n=== Playback URLs ===")
    for url_info in data['playback_urls']:
        print(f"- {url_info['url']}")

    print("\n=== API Requests ===")
    for req in data['radio_api_requests']:
        print(f"- {req['url']}")

    print("\nSniffer completed.")
