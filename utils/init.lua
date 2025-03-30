-- init.lua
box.cfg{
    listen = 3301,
    memtx_memory = 256 * 1024 * 1024, -- 256MB
    log_level = 5,
    pid_file = "tarantool.pid"
}

-- Создаем пользователя (если не существует)
local function create_user_if_not_exists()
    if not box.schema.user.exists('mm_bot') then
        box.schema.user.create('mm_bot', {password = 'securepassword'})
        box.schema.user.grant('mm_bot', 'read,write,execute', 'universe')
        print("User 'mm_bot' created successfully")
    else
        print("User 'mm_bot' already exists")
    end
end

-- Создаем пространство и индексы
local function init_spaces()
    -- Пространство для голосований
    if not box.space.votes then
        box.schema.space.create('votes', {
            format = {
                {name = 'id', type = 'string'},
                {name = 'channel_id', type = 'string'},
                {name = 'creator', type = 'string'},
                {name = 'question', type = 'string'},
                {name = 'options', type = 'map'},
                {name = 'created_at', type = 'unsigned'},
                {name = 'is_active', type = 'boolean'},
                {name = 'voted_users', type = 'map'}
            }
        })

        -- Первичный индекс
        box.space.votes:create_index('primary', {
            type = 'hash',
            parts = {'id'},
            if_not_exists = true
        })
        print("Space 'votes' created successfully")
    end
end

-- Основная инициализация
local function main()
    create_user_if_not_exists()
    init_spaces()

    -- Функции API (остаются без изменений)
    function create_vote(id, channel_id, creator, question, options)
        local vote = {
                id = id,
                channel_id = channel_id,
                creator = creator,
                question = question,
                options = options,
                created_at = os.time(),
                is_active = true
        }
        return box.space.votes:insert(vote)
    end

    function add_vote(vote_id, option, user_id)
        local vote = box.space.votes:get(vote_id)
        if not vote or not vote.is_active then
            return nil, "Vote not found or ended"
        end

        -- Инициализируем voted_users если нет
        vote.voted_users = vote.voted_users or {}

        -- Проверяем, голосовал ли уже
        if vote.voted_users[user_id] ~= nil then
            return nil, "You have already voted"
        end

        if not vote.options[option] then
            return nil, "Invalid option"
        end

        -- Обновляем данные
        vote.options[option] = vote.options[option] + 1
        vote.voted_users[user_id] = true

        box.space.votes:update(vote_id, {
            {'=', 'options', vote.options},
            {'=', 'voted_users', vote.voted_users}
        })

        return vote.options
    end

    function delete_vote(vote_id, user_id)
        local vote = box.space.votes:get(vote_id)
        if vote == nil then
            return nil, "Vote not found"
        end

        -- Проверяем, что пользователь - создатель
        if vote.creator ~= user_id then
            return nil, "Only creator can delete vote"
        end

        return box.space.votes:delete(vote_id)
    end

    function has_user_voted(vote_id, user_id)
        local vote = box.space.votes:get(vote_id)
        if vote == nil then
            return nil, "Vote not found"
        end

        -- Добавляем поле voted_users, если его нет
        vote.voted_users = vote.voted_users or {}

        return vote.voted_users[user_id] ~= nil
    end

    function end_vote(vote_id)
        local vote = box.space.votes:get(vote_id)
        if not vote then
            return nil, "Vote not found"
        end
        local updated_vote = vote:update({
                {'=', 'is_active', false}
            })
        return updated_vote
    end

    function get_results(vote_id)
        local vote = box.space.votes:get(vote_id)
        if not vote then
            return nil, "Vote not found"
        end
        return vote[2]
    end

    function get_active_votes(channel_id)
        local result = {}
        for _, vote in box.space.votes:pairs(channel_id) do
            if vote[2].is_active then
                table.insert(result, vote[2])
            end
        end
        return result
    end

    print("Tarantool voting system initialized successfully")
end

-- Запускаем инициализацию
pcall(main)