# Tablić: Game Logic and Agent Prompt

## 🃏 1. Game Elements & Card Values
* **Deck:** Standard 52-card deck.
* **Numerical Cards (2–10):** Face value.
* **Jack (J):** 12
* **Queen (Q):** 13
* **King (K):** 14
* **Ace (A):** Dynamic value; can be treated as **1 or 11**, depending on what benefits the player during the capture phase.

## 🏆 2. Scoring System (Points)
The total points in a deck (excluding "tablas") sum up to **25**.
* **10, J, Q, K, A:** 1 point each.
* **10 of Diamonds (10♦):** 2 points (known as "Double Ten").
* **2 of Clubs (2♣):** 1 point (known as "Little Two").
* **Most Cards ("Špil"):** The player/team with the most cards at the end of the round gets **3 points**. (If tied, no one gets the points).
* **Tabla:** If a player clears all cards from the table, they earn **1 extra point** (tallied immediately).

## ⚙️ 3. Gameplay Mechanics
* **Dealing:** Deal 6 cards to each player. In the very first deal of the game, place 4 cards face-up on the table.
* **The Move:** A player plays one card from their hand.
    * **Capture:** If the card's value matches a card on the table, or a **sum** of multiple cards on the table, the player captures those cards plus the card they played.
    * **Example:** If there is a 5, 2, and 3 on the table, a player can play a 10 to capture all three ($5 + 2 + 3 = 10$).
    * **Multiple Combinations:** A player can capture multiple sets at once (e.g., playing a 10 to capture a 10, and also a 6+4 combination).
    * **Discard:** If the played card cannot capture anything, it remains face-up on the table.
* **Rounds:** When hands are empty, deal 6 more cards to each player until the deck is exhausted. 
* **The "Last Hand" Rule:** At the end of the final round, any cards remaining on the table go to the player who made the last successful capture.

## 🏁 4. Winning Conditions
* A game is typically played across multiple rounds until a player reaches **101 points**.
* If both players pass 101 in the same round, the player with the higher total score wins.

---

## 🤖 Prompt for the AI Agent

**Role:** Act as a Senior Full Stack Developer. I want you to suggest tech stack for **Tablić** and let us plan togheter.

**Core Requirements:**
1. **State Management:** Create a robust class/structure to handle the deck, player hands, table cards, and the point score for each player.
2. **Capture Algorithm:** Implement a function that calculates if a played card can capture combinations of cards on the table. **Crucial:** Handle the dual value of the Ace (1 or 11) in your summation logic.
3. **Scoring Engine:** Create a function to calculate points at the end of a round. Track specific card points (10♦ = 2 pts, 2♣ = 1 pt, face cards/Aces/10s = 1 pt), award 3 points to the player with the most total cards, and handle "Tabla" points during active play.
4. **Turn Logic:** Ensure players move sequentially. The game must handle 'deals' of 6 cards at a time until the 52-card deck is empty, and apply the 'Last Hand' rule to give remaining table cards to the last capturing player.
5. **Multiplayer Architecture:** Design the backend to support two or four players using WebSockets (or similar real-time tech).
6. **Frontend** Suggest frontend UI and game engine, if we can make this in three.js probably best

lets see what can we do