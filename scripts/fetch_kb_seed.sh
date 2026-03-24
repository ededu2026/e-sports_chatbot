#!/usr/bin/env sh

set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
KB_DIR="$ROOT_DIR/knowledge_base/sources/wikipedia"

mkdir -p "$KB_DIR/games" "$KB_DIR/teams" "$KB_DIR/players"

fetch_summary() {
  slug="$1"
  target="$2"
  if ! curl --retry 4 \
    --retry-delay 2 \
    --retry-all-errors \
    -A "EsportsArenaAI-KBSeeder/1.0" \
    -fsSL "https://en.wikipedia.org/api/rest_v1/page/summary/$slug" \
    -o "$target"; then
    echo "warning: failed to fetch $slug" >&2
  fi
  sleep 1
}

# Games
fetch_summary "Counter-Strike_2" "$KB_DIR/games/counter_strike_2.json"
fetch_summary "Valorant" "$KB_DIR/games/valorant.json"
fetch_summary "League_of_Legends" "$KB_DIR/games/league_of_legends.json"
fetch_summary "Dota_2" "$KB_DIR/games/dota_2.json"
fetch_summary "Rocket_League" "$KB_DIR/games/rocket_league.json"
fetch_summary "Apex_Legends" "$KB_DIR/games/apex_legends.json"
fetch_summary "Overwatch_2" "$KB_DIR/games/overwatch_2.json"
fetch_summary "Tom_Clancy's_Rainbow_Six_Siege" "$KB_DIR/games/rainbow_six_siege.json"
fetch_summary "Mobile_Legends:_Bang_Bang" "$KB_DIR/games/mobile_legends_bang_bang.json"
fetch_summary "PUBG:_Battlegrounds" "$KB_DIR/games/pubg_battlegrounds.json"
fetch_summary "PUBG_Mobile" "$KB_DIR/games/pubg_mobile.json"
fetch_summary "Free_Fire_(video_game)" "$KB_DIR/games/free_fire.json"
fetch_summary "Fortnite_Battle_Royale" "$KB_DIR/games/fortnite_battle_royale.json"
fetch_summary "Call_of_Duty" "$KB_DIR/games/call_of_duty.json"
fetch_summary "StarCraft_II:_Wings_of_Liberty" "$KB_DIR/games/starcraft_ii.json"

# Teams
fetch_summary "Team_Spirit_(esports)" "$KB_DIR/teams/team_spirit.json"
fetch_summary "Team_Vitality" "$KB_DIR/teams/team_vitality.json"
fetch_summary "Natus_Vincere" "$KB_DIR/teams/natus_vincere.json"
fetch_summary "G2_Esports" "$KB_DIR/teams/g2_esports.json"
fetch_summary "FaZe_Clan" "$KB_DIR/teams/faze_clan.json"
fetch_summary "Team_Liquid" "$KB_DIR/teams/team_liquid.json"
fetch_summary "T1_(esports)" "$KB_DIR/teams/t1.json"
fetch_summary "Gen.G" "$KB_DIR/teams/gen_g.json"
fetch_summary "Fnatic" "$KB_DIR/teams/fnatic.json"
fetch_summary "Sentinels_(esports)" "$KB_DIR/teams/sentinels.json"
fetch_summary "Paper_Rex" "$KB_DIR/teams/paper_rex.json"
fetch_summary "EDward_Gaming" "$KB_DIR/teams/edward_gaming.json"
fetch_summary "Bilibili_Gaming" "$KB_DIR/teams/bilibili_gaming.json"

# Players: CS2
fetch_summary "Donk_(gamer)" "$KB_DIR/players/donk.json"
fetch_summary "Sh1ro" "$KB_DIR/players/sh1ro.json"
fetch_summary "ZywOo" "$KB_DIR/players/zywoo.json"
fetch_summary "ApEX_(Counter-Strike)" "$KB_DIR/players/apex_cs.json"
fetch_summary "Ropz" "$KB_DIR/players/ropz.json"
fetch_summary "M0NESY" "$KB_DIR/players/m0nesy.json"
fetch_summary "NiKo_(Counter-Strike_player)" "$KB_DIR/players/niko_cs.json"
fetch_summary "S1mple" "$KB_DIR/players/s1mple.json"
fetch_summary "Aleksib" "$KB_DIR/players/aleksib.json"
fetch_summary "B1t_(gamer)" "$KB_DIR/players/b1t.json"

# Players: League of Legends
fetch_summary "Faker_(gamer)" "$KB_DIR/players/faker.json"
fetch_summary "Chovy" "$KB_DIR/players/chovy.json"
fetch_summary "Keria_(gamer)" "$KB_DIR/players/keria.json"
fetch_summary "Caps_(gamer)" "$KB_DIR/players/caps.json"
fetch_summary "Bin_(gamer)" "$KB_DIR/players/bin.json"
fetch_summary "Knight_(gamer)" "$KB_DIR/players/knight.json"
fetch_summary "Ruler_(gamer)" "$KB_DIR/players/ruler.json"

# Players: VALORANT
fetch_summary "TenZ" "$KB_DIR/players/tenz.json"
fetch_summary "Aspas_(gamer)" "$KB_DIR/players/aspas.json"
fetch_summary "Derke" "$KB_DIR/players/derke.json"
fetch_summary "ZmjjKK" "$KB_DIR/players/zmjjkk.json"
fetch_summary "Meteor_(gamer)" "$KB_DIR/players/meteor.json"

# Players: Other major esports
fetch_summary "N0tail" "$KB_DIR/players/n0tail.json"
fetch_summary "Yatoro" "$KB_DIR/players/yatoro.json"
fetch_summary "ImperialHal" "$KB_DIR/players/imperialhal.json"
fetch_summary "Scump" "$KB_DIR/players/scump.json"
fetch_summary "Serral" "$KB_DIR/players/serral.json"
fetch_summary "SonicFox" "$KB_DIR/players/sonicfox.json"
