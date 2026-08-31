package pairtrans

import (
	"github.com/prizarena/prizarena-public/patrans"
	"github.com/strongo/bots-framework/core"
)

func init() {
	patrans.RegisterTranslations(TRANS)
}

var TRANS = map[string]map[string]string{
	bots.MessageTextOopsSomethingWentWrong: {
		"ru-RU": "Упс, что-то пошло не так... \xF0\x9F\x98\xB3",
		"en-US": "Oops, something went wrong... \xF0\x9F\x98\xB3",
		"fa-IR": "اوه، یک جای کار مشکل دارد ...  \xF0\x9F\x98\xB3",
		"it-IT": "Ops, qualcosa e' andato storto... \xF0\x9F\x98\xB3",
	},
	MT_START_SELECT_LANG: {
		"en-US": "<b>Please select your language</b>\nПожалуйста выберите язык общения",
		"ru-RU": "<b>Пожалуйста выберите язык общения</b>\nPlease select your language",
	},
	FlagOfTheDay: {
		"en-US": `<i>To learn more about flag subscribe to</i> <a href="https://t.me/FlagOfTheDay">@FlagOfTheDay</a> <i>channel</i>.`,
		"ru-RU": `<i>Чтобы узнать больше о флагах подпишитесь на канал</i> <a href="https://t.me/FlagOfTheDay">@FlagOfTheDay</a>.`,
	},
	Flips: {
		"en-US": "<b>Turns</b>: %v",
		"ru-RU": "<b>Ходов</b>: %v",
	},
	SinglePlayerMatchedOne: {
		"en-US": "<b>Matched</b>: 1 pair",
		"ru-RU": "<b>Найдено</b>: 1 пара",
	},
	SinglePlayerMatchedCount: {
		"en-US": "<b>Matched</b>: %v",
		"ru-RU": "<b>Найдено</b>: %v",
	},
	ChallengeFriendCommandText: {
		"en-US": "🤺 Challenge Telegram friend",
		"ru-RU": "🤺 Новая игра в Telegram",
	},
	NewGameInlineTitle: {
		"en-US": "🀄 Pair matching - new game",
		"ru-RU": "🀄 Найди пары - новая игра",
	},
	NewGameInlineDescription: {
		"en-US": "Starts new Pair-Matching game",
		"ru-RU": "Создать новую игру",
	},
	GameCardTitle: {
		"en-US": "Pair-Matching game",
		"ru-RU": "Игра: Найди пару",
	},
	OnStartWelcome: {
		"en-US": `🀄 <b>Pair-Matching game</b>

You are given a board with closed tiles. Find matching pairs by opening tiles 1 by 1. If you open 2 non matching tiles they get closed. 

🤺 You can practice alone or play in race mode against friends. 

🏆 Create tournaments for your friends or subscribers of your channel.

💵 From time to time there are <b>free to play</b> sponsored tournaments with cash prizes! 
`,
		"ru-RU": `🀄 <b>Игра: Найди пару</b>

Создаётся поле с закрытыми карточками. Открывая их по одной найдите пары. Если вы открыли 2 несовпадающие карточки то они закрываются.

🤺 Играть можно одному или на перегонки с друзьями.

🏆 Проводите турниры среди друзей или подписчиков своего канала. 

💵 Иногда проводятся спонсорские турниры с <b>бесплатным участием</b> и денежными призами!
`,
	},
	ChooseSizeOfNextBoard: {
		"en-US": "Choose size of next board:",
		"ru-RU": "Выберите размер следующей доски:",
	},
	SinglePlayer: {
		"en-US": "⚔ Single-player",
		"ru-RU": "⚔ Играть одному",
	},
	MultiPlayer: {
		"en-US": "⚔ Multi-player",
		"ru-RU": "⚔ Играть с противником",
	},
	Board: {
		"en-US": "Board",
		"ru-RU": "Доска",
	},
	Tournaments: {
		"en-US": "🏆 Tournaments",
		"ru-RU": "🏆 Турниры",
	},
	FirstMoveDoneAwaitingSecond: {
		"en-US": "Player <b>%v</b> made choice, awaiting another player...",
		"ru-RU": "Игрок <b>%v</b> сделал свой ход, ожидается ход второго игрока...",
	},
	FindFast: {
		"en-US": "Find matching pairs as fast as you can.",
		"ru-RU": "Найдите совпадающие пары настолько быстро как можете.",
	},
	RulesShort: {
		"en-US": `<pre></pre>`,
	},
	NewGameText: {
		"en-US": `🀄 <b>Pair matching game</b>

Please choose board size.`,
		"ru-RU": `🀄 Игра: <b>Найди пары</b>

Выберите размер доски.`,
	},
	MT_HOW_TO_START_NEW_GAME: {
		"en-US": `<b>To begin new game:</b>
  1. Open Telegram chat with your friend
  2. Type into the text input @BiddingTicTacToeBot and a space
  3. Wait for a popup menu and choose "New game"

<i>First 2 steps can be replaced by clicking the button below!</i>`,
		//
		"ru-RU": `<b>Чтобы начать игру:</b>
  1. Откройте чат с вашим другом
  2. Наберите в поле ввода @BiddingTicTacToeBot и пробел
  3. Дождитесь появлению меню и выберите "Новая игра"

<i>Два первых шага могут быть заменены одним кликом на кнопку ниже!</i>`,
	},
	MT_NEW_GAME_WELCOME: {
		"en-US": `To start the game please choose board size.`,
		"ru-RU": `Чтобы начать игру выберите размер доски.`,
	},
	MT_HOW_TO_INLINE: {
		"en-US": `To begin the game and to make first move:
  * choose a cell
  * click Start at bottom of the screen`,
		//
		"ru-RU": `Чтобы начать игру и сделать первый ход:
  * выберите клетку
  * нажмите Start внизу экрана`,
	},
	MT_PLAYER: {
		"en-US": "Player <b>%v</b>:",
		"ru-RU": "Игрок <b>%v</b>:",
	},
	MT_AWAITING_PLAYER: {
		"en-US": "awaiting...",
		"ru-RU": "ожидается...",
	},
	MT_PLAYER_BALANCE: {
		"en-US": "Balance: %d",
		"ru-RU": "Баланс: %d",
	},
	MT_ASK_TO_RATE: {
		"en-US": `🙋 <b>Did you like the game?</b> We'll appreciate if you <a href="https://t.me/storebot?start=BiddingTicTacToeBot">rate us</a> with 5⭐s!'`,
		"ru-RU": `🙋 <b>Понравилась игра?</b> Будем признательны если <a href="https://t.me/storebot?start=BiddingTicTacToeBot">оцените нас</a> на 5⭐!`,
	},
	// MT_FREE_GAME_SPONSORED_BY: {
	// 	"en-US": "🙏 Free game sponsored by %v",
	// 	"ru-RU": "🙏 Бесплатная игра при поддержке %v - бот для учёта долгов",
	// },
}
